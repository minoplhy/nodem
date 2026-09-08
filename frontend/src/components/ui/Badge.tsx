import React from 'react';

export interface BadgeProps {
  status: 'UP' | 'DOWN' | 'UNKNOWN' | 'DISABLED' | string;
  label?: string;
  className?: string;
}

export const Badge: React.FC<BadgeProps> = ({ status, label, className = '' }) => {
  const norm = status.toUpperCase();
  let variant = 'unknown';

  if (norm === 'UP') variant = 'up';
  else if (norm === 'DOWN') variant = 'down';
  else if (norm === 'DISABLED') variant = 'disabled';

  return (
    <span className={`badge badge-${variant} ${className}`}>
      <span className="badge-dot" />
      {label || norm}
    </span>
  );
};
