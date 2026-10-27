# Security policy

decruft runs cleanup commands, moves folders to the Trash and hands removals to other tools, so any of these is a security issue:

- it removes, moves or changes something outside the rules in the README's [safety model](README.md#safety-model), or follows a symlink out of an item;
- it runs a command that is not the owning tool's documented cleanup, runs `sudo`, or runs anything through a shell;
- it acts on an item that changed since the scan, or on a protected or live item;
- it passes override flags (`--force`, `--unlock`) to a family tool, or writes to another tool's state.

## Reporting

Please report privately through GitHub: **Security → Report a vulnerability** on this repository. Do not open a public issue.

Include the decruft version, OS, the tools involved and steps to reproduce using placeholder paths. You can expect an acknowledgement within a week.

## Supported versions

Only the latest release receives fixes.
