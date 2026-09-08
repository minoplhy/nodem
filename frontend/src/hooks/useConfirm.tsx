import React, { createContext, useContext, useState, useRef, useCallback } from 'react';
import { AlertTriangleIcon } from '../components/icons/Icons';

export interface ConfirmOptions {
  title?: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
}

type ConfirmContextType = (options: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmContextType | null>(null);

export const ConfirmProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [open, setOpen] = useState(false);
  const [options, setOptions] = useState<ConfirmOptions>({ message: '' });
  const resolverRef = useRef<((val: boolean) => void) | null>(null);

  const confirm = useCallback((opts: ConfirmOptions): Promise<boolean> => {
    setOptions(opts);
    setOpen(true);
    return new Promise((resolve) => {
      resolverRef.current = resolve;
    });
  }, []);

  const handleCancel = () => {
    setOpen(false);
    if (resolverRef.current) {
      resolverRef.current(false);
      resolverRef.current = null;
    }
  };

  const handleConfirm = () => {
    setOpen(false);
    if (resolverRef.current) {
      resolverRef.current(true);
      resolverRef.current = null;
    }
  };

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {open && (
        <div className="modal-backdrop visible" onClick={handleCancel}>
          <div
            className="modal-card glass-panel confirm-dialog"
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-modal="true"
          >
            <div className="confirm-header">
              <div className={`confirm-icon-box ${options.danger ? 'danger' : 'warn'}`}>
                <AlertTriangleIcon size={22} />
              </div>
              <h3 className="confirm-title">{options.title || 'Confirm Action'}</h3>
            </div>
            <p className="confirm-message">{options.message}</p>
            <div className="confirm-actions">
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={handleCancel}
              >
                {options.cancelText || 'Cancel'}
              </button>
              <button
                type="button"
                className={`btn ${options.danger ? 'btn-danger' : 'btn-primary'} btn-sm`}
                onClick={handleConfirm}
                autoFocus
              >
                {options.confirmText || 'Confirm'}
              </button>
            </div>
          </div>
        </div>
      )}
    </ConfirmContext.Provider>
  );
};

export function useConfirm() {
  const context = useContext(ConfirmContext);
  if (!context) {
    throw new Error('useConfirm must be used within a ConfirmProvider');
  }
  return context;
}
