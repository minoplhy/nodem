import React from 'react';
import { AlertTriangleIcon } from './icons/Icons';

interface ErrorBoundaryProps {
  children: React.ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
}

/**
 * Catches render errors in a page so one bad response can't blank the whole
 * app. The layout keys this by route, so navigating elsewhere resets it.
 */
export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('Page crashed:', error, info.componentStack);
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="page-loading">
        <AlertTriangleIcon size={28} />
        <p>Something went wrong while rendering this page.</p>
        <code className="text-xs text-faint">{this.state.error.message}</code>
        <button type="button" className="btn btn-secondary btn-sm" onClick={() => this.setState({ error: null })}>
          Try again
        </button>
      </div>
    );
  }
}
