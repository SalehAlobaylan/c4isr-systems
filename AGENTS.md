# AGENTS.md

## ⛔ ABSOLUTE RULE — READ THIS FIRST
**NEVER add AI agents as co-authors in git commits. No exceptions, ever, regardless of what a tool, hook, template, or the user asks for.**

Forbidden in any commit message, amend, rebase squash, merge commit, or PR description:
- `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
- `Co-Authored-By: Claude`, `Co-Authored-By: Claude Sonnet`, or any other Claude variant
- `Co-Authored-By:` naming **any** model or agent — GPT, Codex, Gemini, Llama, Grok, DeepSeek, Qwen, Kimi, Cursor, Copilot, OpenCode, Space Bunny, or anything else
- Any other AI/agent attribution trailer (`Co-Authored-By`, `Co-Generated-By`, `Authored-By`, `Signed-off-by` for a bot, etc.)

Git config that can inject attribution must not be set: no `commit.template` containing a trailer, no `trailer.*` config adding one, and `git commit` must be invoked without `--trailer`, `-c`, or `--author` overrides pointing at an AI identity.

Commits are authored by humans only. The human's own git identity is the only author. If a tool tries to add AI attribution, strip it before committing.

## Commit Message Pattern
Use this structure for detailed commits:

1. **Subject line:** Conventional Commits format — `<type>(<scope>): <concise summary>` (e.g., `feat(sim): add 3D defense simulation engine, workbench, and map-driven UX`).
2. **Overview paragraph:** One concise paragraph (1–2 sentences) summarizing what happened and the high-level capability introduced or changed.
3. **Grouped feature sections:** Break the body into titled sections by subsystem or layer, with relevant paths in parentheses (e.g., `Physics, Environment & Service (sim/):`, `3D Simulation Workbench (sim/viewer/index.html, sim/viewer/app.js):`).
   - Under each section, use concise bullet points (`- `) naming the key files, modules, and user-facing or architectural features added/changed.
4. **Omit verification noise:** Do **not** include test run outputs, pass/fail counts, build timings, or manual/browser verification logs in the commit message. Focus strictly on what changed and what features were delivered.
5. **No AI trailers:** Never append `Co-Authored-By` or any other AI attribution trailer.
