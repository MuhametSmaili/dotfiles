# ------------------------------------------------------------
# Functions
# ------------------------------------------------------------
# LazyGit with lock file handling
lg() {
    if ! command -v lazygit >/dev/null 2>&1; then
        echo "lazygit not installed"
        return 1
    fi

    local git_dir
    git_dir=$(git rev-parse --git-dir 2>/dev/null)
    if [[ $? -eq 0 ]]; then
        local lock_file="${git_dir}/index.lock"
        if [[ -f "$lock_file" ]]; then
            echo "Removing stale lock file: $lock_file"
            rm "$lock_file"
        fi
    fi
    command lazygit "$@"
}

# CodeDiff review with the normal Neovim config; arguments go to Neovim.
cdf() {
    command nvim -c 'CodeDiff --exit-on-close' "$@"
}

# Git diff with fzf preview
gdiff() {
    local args="$*"
    local preview_cmd
    if command -v delta >/dev/null 2>&1; then
        preview_cmd="git diff $args --color=always -- {-1} | delta --side-by-side --width \${FZF_PREVIEW_COLUMNS:-\$COLUMNS}"
    else
        preview_cmd="git diff $args --color=always -- {-1}"
    fi
    git diff "$@" --name-only | fzf -m --ansi --preview "$preview_cmd" \
        --layout=reverse --height=100% --preview-window=down:90%
}

# CD to git root
cdg() {
    local root=$(git rev-parse --show-toplevel 2>/dev/null)
    if [[ -n "$root" ]]; then
        cd "$root"
    else
        echo "Not in a git repository"
    fi
}

# Create directory and cd into it
mkcd() {
    mkdir -p "$1" && cd "$1"
}

# Yazi wrapper that changes the shell's directory on exit
# Use Q instead of q to leave the current directory unchanged.
y() {
    local tmp cwd exit_status
    tmp="$(mktemp -t "yazi-cwd.XXXXXX")" || return 1

    command yazi "$@" --cwd-file="$tmp"
    exit_status=$?

    IFS= read -r -d '' cwd < "$tmp"
    if [[ -n "$cwd" && "$cwd" != "$PWD" && -d "$cwd" ]]; then
        builtin cd -- "$cwd"
    fi

    command rm -f -- "$tmp"
    return "$exit_status"
}

# Extract various archive formats
extract() {
    if [ -f "$1" ]; then
        case "$1" in
            *.tar.bz2) tar xjf "$1" ;;
            *.tar.gz)  tar xzf "$1" ;;
            *.tar.xz)  tar xJf "$1" ;;
            *.bz2)     bunzip2 "$1" ;;
            *.gz)      gunzip "$1" ;;
            *.tar)     tar xf "$1" ;;
            *.tbz2)    tar xjf "$1" ;;
            *.tgz)     tar xzf "$1" ;;
            *.zip)     unzip "$1" ;;
            *.Z)       uncompress "$1" ;;
            *.7z)      7z x "$1" ;;
            *.rar)     unrar x "$1" ;;
            *)         echo "'$1' cannot be extracted via extract()" ;;
        esac
    else
        echo "'$1' is not a valid file"
    fi
}

# Compress a file/folder to a desired archive
compress() {
    if (( $# < 2 )); then
        echo "Usage: compress file_or_dir... archive"
        return 1
    fi

    archive="${argv[-1]}"
    inputs=("${(@)argv[1,-2]}")

    case "$archive" in
        *.tar.bz2|*.tbz2) tar cjf "$archive" -- "${inputs[@]}" ;;
        *.tar.gz|*.tgz)   tar czf "$archive" -- "${inputs[@]}" ;;
        *.tar.xz)        tar cJf "$archive" -- "${inputs[@]}" ;;
        *.tar)           tar cf  "$archive" -- "${inputs[@]}" ;;
        *.zip)           zip -r  "$archive" -- "${inputs[@]}" ;;
        *)
            echo "'$archive' has an unsupported format"
            return 1
            ;;
    esac
}

# Fuzzy find and kill process
fkill() {
    command -v fzf >/dev/null 2>&1 || { echo "Install fzf first!"; return 1; }
    local pid
    pid=$(ps -ef | sed 1d | fzf -m --height=60% --layout=reverse \
            --header='Select process to kill' \
            --preview 'ps -fp {2}' \
        --preview-window=down:40% | awk '{print $2}')

    if [ -n "$pid" ]; then
        echo "$pid" | xargs kill -"${1:-TERM}" || return $?
        echo "Sent ${1:-TERM} to process(es): $pid"
    fi
}

ovi() {
    command -v fzf >/dev/null 2>&1 || { echo "Install fzf first!"; return 1; }
    local file
    file=$(fzf --preview="bat --style=numbers --color=always {}" --height=40% --reverse)
    [[ -n "$file" ]] && nvim "$file"
}

scp_push() {
    local host="${1:-dev_env}"
    local remote_path="${2:-"~/inbox"}"

    echo "Select file(s) to send to $host:$remote_path"
    echo "TAB to mark multiple • ENTER to confirm • CTRL-C to cancel"
    echo

    local files
    files=$(fd -t f -H --exclude .git --exclude node_modules | fzf -m)

    [[ -z "$files" ]] && { echo "No files selected."; return 0; }

    echo
    echo "Sending selected file(s)..."
    printf "%s\n" "$files" | xargs -I {} scp {} "$host:$remote_path" || {
        echo "Error: scp failed"
        return 1
    }

    echo "Done."
}

scp_pull() {
    local host="${1:-dev_env}"
    local remote_path="${2:-"~/inbox"}"
    local local_dest="${3:-"."}"

    echo "Listing files from $host:$remote_path"
    echo "TAB to mark multiple • ENTER to confirm • CTRL-C to cancel"
    echo

    local files
    files=$(ssh -o ConnectTimeout=5 -o BatchMode=yes "$host" \
            "cd ${remote_path} && find . -type f" 2>/dev/null) || {
        echo "Cannot reach $host (timeout or auth required)"
        return 1
    }

    files=$(printf "%s\n" "$files" | fzf -m)
    [[ -z "$files" ]] && { echo "No files selected."; return 0; }

    echo
    echo "Downloading selected file(s)..."
    while IFS= read -r file; do
        scp "$host:$remote_path/$file" "$local_dest"
        done <<< "$files" || {
        echo "Error: scp failed"
        return 1
    }

    echo "Done."
}
