# Claude Code Setup & Context

This file contains setup instructions and context specifically for Claude Code sessions working on the Kaimi project.

## GitHub MCP Server Setup

The project has a GitHub MCP (Model Context Protocol) server configured, which gives Claude Code direct access to GitHub issues, pull requests, and repository data.

### Initial Setup

If you're starting a new Claude Code session and the GitHub MCP server isn't configured, add it with:

```bash
claude mcp add --transport http github https://api.githubcopilot.com/mcp/ \
  --header "Authorization: Bearer YOUR_GITHUB_PAT"
```

Replace `YOUR_GITHUB_PAT` with a GitHub Personal Access Token that has access to the `Mawar2/Kaimi` repository.

### What This Enables

With the GitHub MCP server connected, Claude can:
- Fetch and read GitHub issues directly
- Access pull request details
- Query repository information
- Work with GitHub data without requiring the `gh` CLI

### Verifying the Connection

Run `/mcp` in your Claude Code session to verify the GitHub server is connected.

## Repository Information

- **GitHub Repository**: `Mawar2/Kaimi`
- **Main Branch**: `main`
- **Remote URL**: https://github.com/Mawar2/Kaimi.git

## Project Context

See the following files for detailed project information:
- `PROJECT.md` - Project overview and goals
- `ARCHITECTURE.md` - System architecture and design decisions
- `CONVENTIONS.md` - Coding conventions and standards
- `docs/DEVELOPER_SETUP.md` - Developer environment setup

## Working with Issues

The project uses GitHub issues for task tracking, organized by:
- **Phase labels**: `phase-0`, `phase-1`, `phase-2`, `phase-3`
- **Zone labels**: `zone-1` (Malik), `zone-2` (Timm)
- **Agent labels**: `agent:hunter`, `agent:scorer`, `agent:outline`, `agent:final-review`
- **Team labels**: `malik`, `timm`

Local ticket files are also maintained:
- `kaimi_malik_tickets.md` - Malik's ticket tracking
- `kaimi_timm_tickets.md` - Timm's ticket tracking

## AI Code Review in CI/CD

The project has an **automated AI code review** system integrated into the CI/CD pipeline.

### How It Works

Every pull request triggers an AI code review that:
1. Authenticates to GCP and retrieves the Gemini API key from Secret Manager
2. Gets the PR diff (limited to 50KB)
3. Sends the diff to **Gemini 2.0 Flash** for review
4. AI reviews for:
   - Bugs and logic errors
   - Security vulnerabilities (OWASP Top 10)
   - Performance issues
   - Go best practices and idioms
   - Test coverage gaps
5. Posts review feedback as a PR comment
6. **Required gate** - must complete before merge

### Workflow Integration

This aligns with the team's **Definition of Done**:
> ✓ AI sub-agent review done and its report addressed

- The AI review is **required** but not **blocking** based on findings
- Developer must see the feedback (gate ensures review completes)
- Developer + human reviewer decide what to fix
- Not all AI suggestions must be addressed, but they must be considered

### Technical Details

- **Model**: Gemini 2.0 Flash Experimental (`gemini-2.0-flash-exp`)
- **API Key**: Stored in GCP Secret Manager as `google-ai-studio-api-key`
- **Cost**: Free (Gemini free tier: 15 requests/min)
- **Workflow**: `.github/workflows/ci.yml` - `ai-code-review` job
- **Runs on**: Every PR from non-fork branches (requires GCP secrets)

### For Claude Agents

When working on code changes:
- The AI review will run automatically when you create a PR
- Review its feedback and either fix issues or explain why they're false positives
- Do not skip or bypass this gate - it's part of the team's quality process
- If the review finds critical security issues, prioritize fixing them

## Tips for Claude Sessions

- Always check the current git status and recent commits for context
- Reference issue numbers when making commits (format: `<issue#>_description`)
- Use the GitHub MCP server to fetch fresh issue data when needed
- The project is built in Go and uses Google Cloud Platform (GCP) services
- All PRs will be reviewed by AI - expect and address feedback as part of the workflow
