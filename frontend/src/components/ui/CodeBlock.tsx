import React, { useState } from 'react';
import { CopyIcon, CheckIcon } from '../icons/Icons';

interface CodeBlockProps {
  code: string;
  title?: string;
  language?: string;
  multiLine?: boolean;
  showPrompt?: boolean;
  className?: string;
}

export const CodeBlock: React.FC<CodeBlockProps> = ({
  code,
  title,
  language,
  multiLine = false,
  showPrompt = !multiLine,
  className = '',
}) => {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (multiLine) {
    return (
      <div className={`terminal-code-block ${className}`}>
        <div className="terminal-code-header">
          <div className="terminal-header-left">
            <span className="terminal-dots">
              <span className="terminal-dot terminal-dot-red" />
              <span className="terminal-dot terminal-dot-yellow" />
              <span className="terminal-dot terminal-dot-green" />
            </span>
            {title && <span className="terminal-title">{title}</span>}
          </div>
          <div className="terminal-header-right">
            {language && <span className="terminal-badge">{language.toUpperCase()}</span>}
            <button
              type="button"
              className={`terminal-copy-btn ${copied ? 'copied' : ''}`}
              onClick={handleCopy}
              aria-label="Copy code to clipboard"
            >
              {copied ? <CheckIcon size={13} /> : <CopyIcon size={13} />}
              <span>{copied ? 'Copied' : 'Copy'}</span>
            </button>
          </div>
        </div>
        <div className="terminal-code-body terminal-code-multiline">
          <pre>
            <code>{code}</code>
          </pre>
        </div>
      </div>
    );
  }

  return (
    <div className={`terminal-cmd-bar ${className}`}>
      {title && (
        <div className="terminal-cmd-label">
          <span>{title}</span>
        </div>
      )}
      <div className="terminal-cmd-content">
        <div className="terminal-code-scroll">
          {showPrompt && <span className="terminal-prompt">$</span>}
          <code className="terminal-cmd-text">{code}</code>
        </div>
        <button
          type="button"
          className={`terminal-copy-btn ${copied ? 'copied' : ''}`}
          onClick={handleCopy}
          aria-label="Copy command to clipboard"
        >
          {copied ? <CheckIcon size={13} /> : <CopyIcon size={13} />}
          <span>{copied ? 'Copied' : 'Copy'}</span>
        </button>
      </div>
    </div>
  );
};
