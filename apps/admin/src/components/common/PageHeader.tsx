import React from 'react';

interface PageHeaderProps {
  title: string;
  description?: string;
  children?: React.ReactNode;
}

export function PageHeader({ title, description, children }: PageHeaderProps) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-5 border-b border-[#DCE4DF] mb-6">
      <div>
        <h1 className="text-xl font-bold tracking-tight text-[#17201B] font-sans sm:text-2xl">
          {title}
        </h1>
        {description && (
          <p className="mt-1 text-sm text-[#17201B]/60 max-w-2xl font-sans">
            {description}
          </p>
        )}
      </div>
      {children && (
        <div className="flex items-center gap-2.5 self-start sm:self-center">
          {children}
        </div>
      )}
    </div>
  );
}
