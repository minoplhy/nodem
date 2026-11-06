import React, { createContext, useContext, useState, useCallback, useMemo } from 'react';
import { CheckIcon, XIcon, AlertTriangleIcon, InfoIcon } from '../components/icons/Icons';

export type ToastType = 'success' | 'error' | 'warn' | 'info';

export interface ToastItem {
  id: string;
  type: ToastType;
  message: string;
}

export interface ToastApi {
  success: (message: string) => void;
  error: (message: string) => void;
  warn: (message: string) => void;
  info: (message: string) => void;
}

// Holds only the stable toast API. The toast list lives in provider-local state,
// so showing/expiring a toast never changes this context value and never
// re-renders (or re-triggers effects in) consumers.
const ToastContext = createContext<ToastApi | null>(null);

export const ToastProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const addToast = useCallback((type: ToastType, message: string) => {
    const id = Math.random().toString(36).substring(2, 9);
    setToasts((prev) => [...prev, { id, type, message }]);

    setTimeout(() => {
      removeToast(id);
    }, 4000);
  }, [removeToast]);

  const toast = useMemo<ToastApi>(
    () => ({
      success: (msg: string) => addToast('success', msg),
      error: (msg: string) => addToast('error', msg),
      warn: (msg: string) => addToast('warn', msg),
      info: (msg: string) => addToast('info', msg),
    }),
    [addToast]
  );

  return (
    <ToastContext.Provider value={toast}>
      {children}
      <div className="toast-container" aria-live="polite">
        {toasts.map((t) => (
          <div key={t.id} className={`toast-item toast-${t.type}`}>
            <div className="toast-icon">
              {t.type === 'success' && <CheckIcon size={18} />}
              {t.type === 'error' && <XIcon size={18} />}
              {t.type === 'warn' && <AlertTriangleIcon size={18} />}
              {t.type === 'info' && <InfoIcon size={18} />}
            </div>
            <div className="toast-message">{t.message}</div>
            <button
              className="toast-close"
              onClick={() => removeToast(t.id)}
              aria-label="Close notification"
            >
              <XIcon size={14} />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
};

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return context;
}
