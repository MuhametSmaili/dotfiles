import assert from "node:assert/strict";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test, { type TestContext } from "node:test";
import {
	CustomEditor, type KeybindingsManager,
	type ExtensionAPI, type ExtensionContext, type Theme,
} from "@earendil-works/pi-coding-agent";
import { CombinedAutocompleteProvider, KeybindingsManager as TuiKeybindingsManager, type TUI } from "@earendil-works/pi-tui";
import leaderExtension from "../index.ts";

const ESC = "\x1b";

function setup(t: TestContext) {
	const agentDir = mkdtempSync(join(tmpdir(), "pi-leader-test-"));
	const previousAgentDir = process.env.PI_CODING_AGENT_DIR;
	process.env.PI_CODING_AGENT_DIR = agentDir;
	t.after(() => {
		if (previousAgentDir === undefined) delete process.env.PI_CODING_AGENT_DIR;
		else process.env.PI_CODING_AGENT_DIR = previousAgentDir;
		rmSync(agentDir, { recursive: true, force: true });
	});

	t.mock.timers.enable({ apis: ["Date", "setTimeout"], now: 10_000 });
	const handlers = new Map<string, (event: never, ctx: ExtensionContext) => unknown>();
	leaderExtension({
		on: (name: string, handler: (event: never, ctx: ExtensionContext) => unknown) => handlers.set(name, handler),
		registerCommand: () => {},
	} as unknown as ExtensionAPI);

	const keybindings = new TuiKeybindingsManager({
		"app.interrupt": { defaultKeys: "escape" },
	}) as KeybindingsManager;
	const theme = {
		fg: (_color: string, text: string) => text,
		bold: (text: string) => text,
		borderColor: (text: string) => text,
		selectList: {
			selectedPrefix: (text: string) => text,
			selectedText: (text: string) => text,
			description: (text: string) => text,
			scrollInfo: (text: string) => text,
			noMatch: (text: string) => text,
		},
	} as unknown as Theme & ConstructorParameters<typeof CustomEditor>[1];
	const tui = { terminal: { rows: 30 }, requestRender: () => {} } as unknown as TUI;
	let idle = false;
	let editor: CustomEditor | undefined;
	let interrupts = 0;
	let status: string | undefined;
	const ctx = {
		mode: "tui",
		isIdle: () => idle,
		ui: {
			theme,
			setWidget: () => {},
			setStatus: (_key: string, value: string | undefined) => { status = value; },
			setEditorComponent: (factory?: (tui: TUI, theme: Theme, kb: KeybindingsManager) => CustomEditor) => {
				editor = factory?.(tui, theme, keybindings);
				if (editor) editor.onEscape = () => { interrupts++; };
			},
		},
	} as unknown as ExtensionContext;
	const emit = (name: string) => handlers.get(name)?.({} as never, ctx);
	emit("session_start");
	assert.ok(editor);
	t.after(() => emit("session_shutdown"));

	return {
		editor,
		emit,
		interrupts: () => interrupts,
		status: () => status,
		setIdle: (value: boolean) => { idle = value; },
	};
}

test("requires two quick Escape presses before forwarding the native interrupt", (t) => {
	const app = setup(t);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 0);
	t.mock.timers.tick(250);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 1);
	assert.equal(app.status(), undefined);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 1, "a third press starts a new pair");
});

for (const delay of [600, 1_000]) {
	test(`requires a fresh pair after ${delay} ms and removes the hint`, (t) => {
		const app = setup(t);
		app.editor.handleInput(ESC);
		assert.equal(app.status(), "Press Esc again to abort");
		t.mock.timers.tick(delay);
		assert.equal(app.status(), undefined);
		app.editor.handleInput(ESC);
		assert.equal(app.interrupts(), 0);
		t.mock.timers.tick(599);
		app.editor.handleInput(ESC);
		assert.equal(app.interrupts(), 1);
	});
}

test("typing between Escapes cancels confirmation without losing input", (t) => {
	const app = setup(t);
	app.editor.handleInput(ESC);
	app.editor.handleInput("x");
	assert.equal(app.editor.getText(), "x");
	assert.equal(app.status(), undefined);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 0);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 1);
});

test("passes idle Escapes through for Pi's native double-Escape navigation", (t) => {
	const app = setup(t);
	app.setIdle(true);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 1);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 2);
	assert.equal(app.status(), undefined);
});

test("one Escape closes leader hints without arming an interrupt", (t) => {
	const app = setup(t);
	app.editor.handleInput(ESC);
	app.editor.handleInput("\x18"); // Default leader: Ctrl+X.
	assert.equal(app.status(), undefined);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 0);
	assert.equal(app.status(), undefined);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 0);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 1);
});

test("one Escape closes autocomplete without arming an interrupt", async (t) => {
	const app = setup(t);
	app.editor.setAutocompleteProvider(new CombinedAutocompleteProvider([
		{ name: "help" }, { name: "hello" },
	], tmpdir()));
	app.editor.handleInput("/");
	t.mock.timers.tick(200);
	await new Promise<void>((resolve) => setImmediate(resolve));
	assert.equal(app.editor.isShowingAutocomplete(), true);
	app.editor.handleInput(ESC);
	assert.equal(app.editor.isShowingAutocomplete(), false);
	assert.equal(app.interrupts(), 0);
	assert.equal(app.status(), undefined);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 0);
	app.editor.handleInput(ESC);
	assert.equal(app.interrupts(), 1);
});

test("Kitty Escape releases and key repeats do not confirm an abort", (t) => {
	const app = setup(t);
	app.editor.handleInput("\x1b[27u");
	app.editor.handleInput("\x1b[27;1:2u"); // Held-key repeat.
	app.editor.handleInput("\x1b[27;1:3u"); // Release.
	assert.equal(app.interrupts(), 0);
	app.editor.handleInput("\x1b[27u");
	assert.equal(app.interrupts(), 1);
});

for (const event of ["agent_start", "agent_end", "session_shutdown"]) {
	test(`${event} clears pending confirmation and its hint`, (t) => {
		const app = setup(t);
		app.editor.handleInput(ESC);
		app.emit(event);
		assert.equal(app.status(), undefined);
		app.editor.handleInput(ESC);
		assert.equal(app.interrupts(), 0);
	});
}
