import React from 'react';
import { LucideIcon, Inbox } from 'lucide-react';

interface EmptyStateProps {
  title: string;
  description: string;
  icon?: LucideIcon;
  action?: React.ReactNode;
}

export function EmptyState({ title, description, icon: Icon = Inbox, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center text-center p-8 border border-dashed border-[#DCE4DF] rounded-md bg-white min-h-[220px]">
      <Icon className="w-10 h-10 text-[#17201B]/35 stroke-[1.5] mb-3" />
      <h3 className="text-sm font-semibold text-[#17201B] font-sans">
        {title}
      </h3>
      <p className="mt-1 text-xs text-[#17201B]/50 max-w-xs font-sans">
        {description}
      </p>
      {action && (
        <div className="mt-4">
          {action}
        </div>
      )}
    </div>
  );
}
