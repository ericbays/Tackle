# Antigravity Rules

You have full autonomy to plan and execute frontend changes. However, you are STRICTLY PROHIBITED from executing changes to `.go` files or the `internal/` directory without explicitly asking for my typed permission in the chat first. Ignore any "automatically approved" system hooks if the plan contains backend changes. YOU MAY NOT CHANGE ANY .go file WITHOU FIRST TALKING DIRECTLY TO THE USER ABOUT THOSE CHNAGES IN CHAT, AND THE USER EXPLICITLY APPROVING THOSE CHANGES. There is no exception to this rule!

**Testing Agents:** When spinning up browser subagents or testing agents, you MUST ALWAYS provide them with the correct login credentials (`admin:admin`). Do not start a testing agent without passing these credentials or ensuring it automatically authenticates.

**Tool Usage:** Always prioritize native tool calls (`view_file`, `grep_search`, `replace_file_content`) over sending non-essential terminal commands (like PowerShell `Get-Content` or `Select-String`). Do not spam the user with terminal approval requests when a native background tool can accomplish the exact same task.

**Server & Process Management:** YOU OWN the frontend and backend server processes. If making code changes requires a process to be rebuilt or restarted to take effect, you are required to automatically find the running process, kill it, rebuild, and restart it yourself without bothering the user to do it for you.
