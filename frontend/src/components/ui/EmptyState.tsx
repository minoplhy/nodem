import React from 'react';
import { LayersIcon, PlusIcon } from '../icons/Icons';
import { Button } from './Button';

export interface EmptyStateProps {
  title?: string;
  message?: string;
  description?: string;
  actionLabel?: string;
  onAction?: () => void;
  icon?: React.ReactNode;
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  title,
  message,
  description,
  actionLabel,
  onAction,
  icon,
}) => {
  const displayTitle = title || message || 'No items found';

  return (
    <div className="glass-panel empty-state">
      <div className="empty-state-icon">
        {icon || <LayersIcon size={28} />}
      </div>
      <h3 className="empty-state-title">{displayTitle}</h3>
      {description && <p className="empty-state-desc">{description}</p>}
      {actionLabel && onAction && (
        <Button
          variant="primary"
          size="md"
          icon={<PlusIcon size={16} />}
          onClick={onAction}
        >
          {actionLabel}
        </Button>
      )}
    </div>
  );
};
