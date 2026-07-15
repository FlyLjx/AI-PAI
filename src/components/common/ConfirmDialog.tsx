import React from 'react';
import { AlertTriangle, Info, ShieldAlert } from 'lucide-react';

interface ConfirmDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  description: string;
  confirmText?: string;
  cancelText?: string;
  type?: 'info' | 'warning' | 'danger';
}

export function ConfirmDialog({
  isOpen,
  onClose,
  onConfirm,
  title,
  description,
  confirmText = '确认',
  cancelText = '取消',
  type = 'info'
}: ConfirmDialogProps) {
  if (!isOpen) return null;

  const getTheme = () => {
    switch (type) {
      case 'danger':
        return {
          icon: ShieldAlert,
          iconColor: 'text-[#DC2626]',
          iconBg: 'bg-red-50',
          btnBg: 'bg-[#DC2626] hover:bg-[#DC2626]/90 text-white',
          border: 'border-l-4 border-l-[#DC2626]'
        };
      case 'warning':
        return {
          icon: AlertTriangle,
          iconColor: 'text-[#D97706]',
          iconBg: 'bg-amber-50',
          btnBg: 'bg-[#D97706] hover:bg-[#D97706]/90 text-white',
          border: 'border-l-4 border-l-[#D97706]'
        };
      default:
        return {
          icon: Info,
          iconColor: 'text-blue-600',
          iconBg: 'bg-blue-50',
          btnBg: 'bg-zinc-900 hover:bg-zinc-800 text-white',
          border: 'border-l-4 border-l-blue-600'
        };
    }
  };

  const theme = getTheme();
  const Icon = theme.icon;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/45 backdrop-blur-[1px]">
      {/* Dialog Body */}
      <div
        className={`bg-white rounded-md max-w-md w-full border border-[#DCE4DF] shadow-xl overflow-hidden ${theme.border} transform transition-all animate-in fade-in zoom-in-95 duration-150`}
        role="dialog"
        aria-modal="true"
      >
        <div className="p-5 flex gap-4">
          <div className={`flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${theme.iconBg}`}>
            <Icon className={`w-5.5 h-5.5 ${theme.iconColor}`} />
          </div>
          <div className="flex-1">
            <h3 className="text-base font-bold text-[#17201B] font-sans">
              {title}
            </h3>
            <p className="mt-1.5 text-xs text-[#17201B]/60 leading-relaxed font-sans">
              {description}
            </p>
          </div>
        </div>

        {/* Buttons footer */}
        <div className="bg-[#F6F8F6] px-5 py-3.5 flex items-center justify-end gap-2.5 border-t border-[#DCE4DF]">
          <button
            type="button"
            onClick={onClose}
            className="px-3.5 py-1.5 text-xs font-semibold text-[#17201B]/70 bg-white border border-[#DCE4DF] rounded-md hover:bg-zinc-50 active:bg-zinc-100 transition-colors font-sans"
          >
            {cancelText}
          </button>
          <button
            type="button"
            onClick={() => {
              onConfirm();
              onClose();
            }}
            className={`px-3.5 py-1.5 text-xs font-semibold rounded-md transition-colors font-sans ${theme.btnBg}`}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
