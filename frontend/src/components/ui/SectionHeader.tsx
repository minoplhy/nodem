import React from 'react';

export interface SectionHeaderProps {
  title: string;
  subtitle?: string;
  children?: React.ReactNode;
  actions?: React.ReactNode;
}

export const SectionHeader: React.FC<SectionHeaderProps> = ({
  title,
  subtitle,
  children,
  actions,
}) => {
  return (
    <header className="section-header">
      <div className="section-title-group">
        <h1 className="section-title font-title">{title}</h1>
        {subtitle && <p className="section-subtitle">{subtitle}</p>}
      </div>
      <div className="header-actions">
        {children}
        {actions}
      </div>
    </header>
  );
};
