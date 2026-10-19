import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { CustomEditor, getAgentDir, type ExtensionAPI, type ExtensionContext, type KeybindingsManager, type Theme } from "@earendil-works/pi-coding-agent";
import { isKeyRelease, isKeyRepeat, Key, matchesKey, parseKey, type KeyId, type TUI } from "@earendil-works/pi-tui";

const INTERRUPT_WINDOW_MS = 600;

interface LeaderConfig {
	leader?: unknown;
	timeoutMs?: unknown;
	bindings?: unknown;
}

type LeaderBindings = Record<string, string>;

type LeaderTheme = Theme;

const DEFAULT_CONFIG = {
	leader: "ctrl+x",
	timeoutMs: 1500,
	bindings: {
		m: "/model",
		"shift+m": "/mcp",
		s: "/skills",
		u: "/usage",
		r: "/resume",
		t: "/tree",
		n: "/new",
		f: "/fork",
	},
} as const;

export default function leaderExtension(pi: ExtensionAPI): void {
	const config = loadConfig();
	let currentContext: ExtensionContext | undefined;
	let activeEditor: LeaderEditor | undefined;

	function dispatch(command: string): void {
		const ctx = currentContext;
		const editor = activeEditor;
		if (!ctx || !editor) return;
		const message = command.startsWith("/") ? command : `/${command}`;
		if (!editor.submitCommand(message)) {
			ctx.ui.notify("Leader action could not submit because the editor is not ready", "error");
		}
	}

	function setHints(_editor: LeaderEditor, pending: boolean, theme: LeaderTheme | undefined): void {
		if (!currentContext || !pending || !theme) {
			currentContext?.ui.setWidget("leader-key", undefined);
			return;
		}

		const hints = Object.entries(config.bindings)
			.map(([key, command]) => `${theme.fg("accent", key)} ${theme.fg("muted", labelForCommand(command))}`)
			.join("   ");
		currentContext.ui.setWidget("leader-key", [
			theme.fg("accent", theme.bold("Leader")) + "  " + hints,
			theme.fg("dim", `press a key · esc cancel · timeout ${config.timeoutMs}ms`),
		]);
	}

	function installEditor(ctx: ExtensionContext): void {
		currentContext = ctx;
		ctx.ui.setEditorComponent((tui, theme, keybindings) => {
			activeEditor = new LeaderEditor(
				tui,
				theme,
				keybindings,
				config,
				dispatch,
				(pending) => setHints(activeEditor!, pending, ctx.ui.theme),
				() => !ctx.isIdle(),
				(pending) => ctx.ui.setStatus("double-escape", pending ? "Press Esc again to abort" : undefined),
			);
			return activeEditor;
		});
	}

	pi.registerCommand("leader", {
		description: "Show the configured leader-key bindings",
		handler: async (_args, ctx) => {
			if (ctx.mode !== "tui") {
				ctx.ui.notify("/leader requires interactive TUI mode", "error");
				return;
			}
			ctx.ui.notify(
				`${config.leader}: ${Object.entries(config.bindings).map(([key, command]) => `${key}=${command}`).join(", ")}`,
				"info",
			);
		},
	});

	pi.on("session_start", (_event, ctx) => {
		if (ctx.mode === "tui") installEditor(ctx);
	});

	pi.on("agent_start", () => activeEditor?.resetInterrupt());
	pi.on("agent_end", () => activeEditor?.resetInterrupt());

	pi.on("session_shutdown", () => {
		activeEditor?.dispose();
		activeEditor = undefined;
		currentContext?.ui.setWidget("leader-key", undefined);
		currentContext?.ui.setEditorComponent(undefined);
		currentContext = undefined;
	});
}

class LeaderEditor extends CustomEditor {
	private pending = false;
	private timeout: ReturnType<typeof setTimeout> | undefined;
	private interruptTimeout: ReturnType<typeof setTimeout> | undefined;
	private lastEscapeTime: number | undefined;

	constructor(
		tui: TUI,
		theme: ConstructorParameters<typeof CustomEditor>[1],
		private readonly appKeybindings: KeybindingsManager,
		private readonly config: { leader: KeyId; timeoutMs: number; bindings: LeaderBindings },
		private readonly dispatch: (command: string) => void,
		private readonly onPendingChange: (pending: boolean) => void,
		private readonly isBusy: () => boolean,
		private readonly onInterruptPendingChange: (pending: boolean) => void,
	) {
		super(tui, theme, appKeybindings);
	}

	override handleInput(data: string): void {
		// Kitty releases/repeats must not count as a second physical Esc press.
		if (isKeyRelease(data)) return;
		const escape = matchesKey(data, Key.escape);
		if (escape && isKeyRepeat(data)) return;

		const guardInterrupt = escape && this.appKeybindings.matches(data, "app.interrupt")
			&& this.isBusy() && !this.pending && !this.isShowingAutocomplete();
		if (!guardInterrupt) this.resetInterrupt();
		if (!this.pending && matchesKey(data, Key.ctrl("l"))) {
			this.setText("");
			this.tui.requestRender(true);
			return;
		}

		if (!this.pending && matchesLeaderKey(data, this.config.leader)) {
			this.enterPending();
			return;
		}

		if (this.pending) {
			if (matchesKey(data, Key.escape)) {
				this.clearPending();
				return;
			}

			const key = normalizeKey(data);
			const command = this.config.bindings[key];
			this.clearPending();
			if (command) this.dispatch(command);
			return;
		}

		if (guardInterrupt) {
			const now = Date.now();
			const confirmed = this.lastEscapeTime !== undefined && now - this.lastEscapeTime < INTERRUPT_WINDOW_MS;
			this.resetInterrupt();
			if (!confirmed) {
				this.lastEscapeTime = now;
				this.onInterruptPendingChange(true);
				this.interruptTimeout = setTimeout(() => this.resetInterrupt(), INTERRUPT_WINDOW_MS);
				this.interruptTimeout.unref?.();
				return;
			}
		}

		// Keep Pi's dynamic abort handler, including queued-message restoration,
		// compaction/retry cancellation, and the native idle double-Esc action.
		super.handleInput(data);
	}

	submitCommand(command: string): boolean {
		if (!this.onSubmit) return false;
		// Pi's submit callback already receives the command text. Do not put the
		// synthetic slash command into the editor, otherwise it can remain visible
		// after the command opens its UI.
		this.onSubmit(command);
		return true;
	}

	resetInterrupt(): void {
		if (this.interruptTimeout) clearTimeout(this.interruptTimeout);
		this.interruptTimeout = undefined;
		if (this.lastEscapeTime === undefined) return;
		this.lastEscapeTime = undefined;
		this.onInterruptPendingChange(false);
	}

	dispose(): void {
		this.clearPending();
		this.resetInterrupt();
	}

	private enterPending(): void {
		this.pending = true;
		this.onPendingChange(true);
		this.timeout = setTimeout(() => this.clearPending(), this.config.timeoutMs);
		this.timeout.unref?.();
	}

	private clearPending(): void {
		if (this.timeout) clearTimeout(this.timeout);
		this.timeout = undefined;
		if (!this.pending) return;
		this.pending = false;
		this.onPendingChange(false);
	}
}

function loadConfig(): { leader: KeyId; timeoutMs: number; bindings: LeaderBindings } {
	const configPath = join(getAgentDir(), "leader.json");
	let parsed: LeaderConfig = {};
	try {
		if (existsSync(configPath)) parsed = JSON.parse(readFileSync(configPath, "utf8")) as LeaderConfig;
	} catch (error) {
		console.error(`Failed to read ${configPath}: ${error instanceof Error ? error.message : String(error)}`);
	}

	const leader = typeof parsed.leader === "string" && parsed.leader.trim() ? parsed.leader.trim() : DEFAULT_CONFIG.leader;
	const timeoutMs = typeof parsed.timeoutMs === "number" && Number.isFinite(parsed.timeoutMs)
		? Math.max(250, Math.min(5_000, Math.floor(parsed.timeoutMs)))
		: DEFAULT_CONFIG.timeoutMs;
	const bindings: LeaderBindings = { ...DEFAULT_CONFIG.bindings };
	if (parsed.bindings && typeof parsed.bindings === "object" && !Array.isArray(parsed.bindings)) {
		for (const [key, command] of Object.entries(parsed.bindings)) {
			if (isBindingKey(key) && typeof command === "string" && command.trim()) bindings[key] = command.trim();
		}
	}
	return { leader: leader as KeyId, timeoutMs, bindings };
}

function matchesLeaderKey(data: string, leader: KeyId): boolean {
	// matchesKey handles legacy and Kitty keyboard protocol sequences. The raw
	// NUL fallback is needed for terminals/tmux configurations that translate
	// Ctrl+Space before the protocol decoder sees it.
	return matchesKey(data, leader) || (leader === "ctrl+space" && data === "\x00");
}

function normalizeKey(data: string): string {
	if (data.length === 1 && data >= "A" && data <= "Z") return `shift+${data.toLocaleLowerCase()}`;
	const parsed = parseKey(data);
	if (parsed) return parsed.toLocaleLowerCase();
	if (data === " ") return "space";
	return data.toLocaleLowerCase();
}

function isBindingKey(key: string): boolean {
	return key.length > 0 && !/\s/u.test(key);
}

function labelForCommand(command: string): string {
	return command.replace(/^\//, "");
}
