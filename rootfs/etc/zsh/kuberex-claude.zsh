# Resumes, at the first prompt of a Rex block's shell, the Claude Code session
# the block ran before its Rex server restarted; see kuberex-claude.
if [[ -n $REX_BLOCK ]]; then
  autoload -Uz add-zsh-hook
  _kuberex_claude_resume() {
    add-zsh-hook -d precmd _kuberex_claude_resume
    /usr/local/bin/kuberex-claude resume
  }
  add-zsh-hook precmd _kuberex_claude_resume
fi
