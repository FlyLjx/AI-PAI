import React from 'react';

type StatusType = 'queued' | 'processing' | 'succeeded' | 'failed' | 'active' | 'disabled' | 'banned' | 'recharge' | 'refund' | 'manual_adjust' | 'pay-as-you-go' | 'subscription';

interface StatusBadgeProps {
  status: StatusType;
  customLabel?: string;
}

export function StatusBadge({ status, customLabel }: StatusBadgeProps) {
  const getStyles = () => {
    switch (status) {
      // Order / Key Statuses
      case 'queued':
        return {
          bg: 'bg-zinc-100',
          text: 'text-zinc-700',
          border: 'border-zinc-300',
          dot: 'bg-zinc-500',
          label: '排队中'
        };
      case 'processing':
        return {
          bg: 'bg-amber-50',
          text: 'text-[#D97706]',
          border: 'border-amber-200',
          dot: 'bg-[#D97706]',
          label: '处理中'
        };
      case 'succeeded':
      case 'active':
        return {
          bg: 'bg-emerald-50',
          text: 'text-[#12B76A]',
          border: 'border-emerald-200',
          dot: 'bg-[#12B76A]',
          label: status === 'active' ? '已启用' : '生成成功'
        };
      case 'failed':
      case 'disabled':
      case 'banned':
        return {
          bg: 'bg-red-50',
          text: 'text-[#DC2626]',
          border: 'border-red-200',
          dot: 'bg-[#DC2626]',
          label: status === 'failed' ? '生成失败' : status === 'disabled' ? '已禁用' : '已封禁'
        };

      // Recharge Logs / Billing Modes
      case 'recharge':
        return {
          bg: 'bg-[#12B76A]/10',
          text: 'text-[#12B76A]',
          border: 'border-[#12B76A]/20',
          dot: 'bg-[#12B76A]',
          label: '余额充值'
        };
      case 'refund':
        return {
          bg: 'bg-amber-50',
          text: 'text-[#D97706]',
          border: 'border-amber-200',
          dot: 'bg-[#D97706]',
          label: '自动退款'
        };
      case 'manual_adjust':
        return {
          bg: 'bg-blue-50',
          text: 'text-blue-700',
          border: 'border-blue-200',
          dot: 'bg-blue-600',
          label: '系统调账'
        };
      case 'pay-as-you-go':
        return {
          bg: 'bg-[#12B76A]/10',
          text: 'text-[#12B76A]',
          border: 'border-[#12B76A]/20',
          dot: 'bg-[#12B76A]',
          label: '按量付费'
        };
      case 'subscription':
        return {
          bg: 'bg-[#0891B2]/10',
          text: 'text-[#0891B2]',
          border: 'border-[#0891B2]/20',
          dot: 'bg-[#0891B2]',
          label: '订阅计费'
        };
      default:
        return {
          bg: 'bg-zinc-100',
          text: 'text-zinc-700',
          border: 'border-zinc-300',
          dot: 'bg-zinc-500',
          label: String(status)
        };
    }
  };

  const style = getStyles();

  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded border text-[11px] font-medium leading-4 ${style.bg} ${style.text} ${style.border}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${style.dot}`}></span>
      <span>{customLabel || style.label}</span>
    </span>
  );
}
