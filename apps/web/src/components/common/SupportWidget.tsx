'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  Check,
  Copy,
  ExternalLink,
  Headset,
  Mail,
  MessageCircle,
  Users,
  X,
} from 'lucide-react';

type SupportSettings = {
  supportEnabled?: boolean | string | number;
  supportTitle?: string;
  supportDescription?: string;
  supportWechat?: string;
  supportQq?: string;
  supportGroupNumber?: string;
  supportGroupUrl?: string;
  supportEmail?: string;
  supportUrl?: string;
  supportQrCodeUrl?: string;
};

type SupportItem = {
  key: string;
  label: string;
  value: string;
  icon: typeof MessageCircle;
  href?: string;
};

function text(value: unknown): string {
  return String(value || '').trim();
}

function enabled(value: unknown): boolean {
  return value === undefined || value === null || value === true || value === 'true' || value === 1 || value === '1';
}

function externalUrl(value: string): string | undefined {
  try {
    const url = new URL(value);
    return ['http:', 'https:'].includes(url.protocol) ? url.toString() : undefined;
  } catch {
    return undefined;
  }
}

export function SupportWidget() {
  const [settings, setSettings] = useState<SupportSettings>({});
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState('');

  useEffect(() => {
    let active = true;
    void fetch('/api/backend/api/settings/public', { cache: 'no-store' })
      .then((response) => response.ok ? response.json() : null)
      .then((payload: { data?: SupportSettings } | null) => {
        if (active && payload?.data) setSettings(payload.data);
      })
      .catch(() => {
        // The button remains available so users still see where support is configured.
      });
    return () => { active = false; };
  }, []);

  const items = useMemo<SupportItem[]>(() => {
    const groupNumber = text(settings.supportGroupNumber);
    const groupUrl = externalUrl(text(settings.supportGroupUrl));
    const email = text(settings.supportEmail);
    const onlineUrl = externalUrl(text(settings.supportUrl));
    return [
      { key: 'wechat', label: '微信客服', value: text(settings.supportWechat), icon: MessageCircle },
      { key: 'qq', label: 'QQ 客服', value: text(settings.supportQq), icon: MessageCircle },
      { key: 'group', label: '群聊群号', value: groupNumber || (groupUrl ? '点击加入群聊' : ''), icon: Users, href: groupUrl },
      { key: 'email', label: '客服邮箱', value: email, icon: Mail, href: email ? `mailto:${email}` : undefined },
      { key: 'online', label: '在线客服', value: onlineUrl ? '打开在线客服' : '', icon: Headset, href: onlineUrl },
    ].filter((item) => item.value);
  }, [settings]);

  if (!enabled(settings.supportEnabled)) return null;

  const copyValue = async (value: string, key: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(key);
      window.setTimeout(() => setCopied((current) => current === key ? '' : current), 1500);
    } catch {
      // Clipboard access can be unavailable in an embedded browser.
    }
  };

  return (
    <>
      <button
        type="button"
        aria-label="联系客服"
        onClick={() => setOpen(true)}
        className="fixed bottom-20 right-4 z-40 inline-flex min-h-10 items-center gap-2 rounded-md border border-[#cce8d6] bg-white px-3 text-xs font-bold text-[#087443] shadow-[0_10px_26px_rgba(15,60,32,.14)] transition hover:border-[#86efac] hover:bg-[#f0fdf4] sm:bottom-6 sm:right-6"
      >
        <Headset size={16} />
        <span>联系客服</span>
      </button>

      {open && (
        <div
          className="fixed inset-0 z-[90] grid place-items-center bg-[rgba(15,23,18,.46)] p-4"
          role="presentation"
          onMouseDown={(event) => { if (event.target === event.currentTarget) setOpen(false); }}
        >
          <section className="w-full max-w-[460px] overflow-hidden rounded-lg border border-[#dce4df] bg-white shadow-[0_22px_60px_rgba(15,30,20,.2)]" role="dialog" aria-modal="true" aria-labelledby="support-dialog-title">
            <div className="flex items-start justify-between gap-4 border-b border-[#edf0ee] bg-[#fafbf9] px-5 py-4">
              <div className="min-w-0">
                <span className="block text-[10px] font-extrabold uppercase tracking-[.14em] text-[#087443]">Support</span>
                <h2 id="support-dialog-title" className="mt-1 text-base font-extrabold text-[#17201b]">{text(settings.supportTitle) || '联系客服'}</h2>
                <p className="mt-1 text-xs leading-5 text-[#6f7a73]">{text(settings.supportDescription) || '遇到充值、生成或账号问题，可以通过下面方式联系管理员。'}</p>
              </div>
              <button type="button" aria-label="关闭客服窗口" onClick={() => setOpen(false)} className="grid h-8 w-8 shrink-0 place-items-center rounded-md border border-[#dce4df] bg-white text-[#748078] hover:border-[#a7dabb] hover:text-[#087443]"><X size={16} /></button>
            </div>
            <div className="space-y-3 px-5 py-5">
              {settings.supportQrCodeUrl && (
                // The QR URL is configured by an administrator and may be a data URL or a customer-hosted image.
                // eslint-disable-next-line @next/next/no-img-element
                <img src={settings.supportQrCodeUrl} alt="客服二维码" className="mx-auto max-h-48 w-auto rounded-md border border-[#edf0ee] object-contain p-2" />
              )}
              {items.length > 0 ? (
                <div className="space-y-2">
                  {items.map((item) => {
                    const Icon = item.icon;
                    return (
                      <div key={item.key} className="flex min-w-0 items-center gap-3 rounded-md border border-[#edf0ee] bg-[#fafbf9] px-3 py-3">
                        <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-[#eaf8ef] text-[#087443]"><Icon size={16} /></span>
                        <div className="min-w-0 flex-1"><span className="block text-[10px] text-[#8a938e]">{item.label}</span><strong className="mt-0.5 block truncate text-xs text-[#26322b]">{item.value}</strong></div>
                        {item.href ? <a className="inline-flex shrink-0 items-center gap-1 text-[11px] font-bold text-[#087443] hover:text-[#065f46]" href={item.href} target="_blank" rel="noreferrer">打开 <ExternalLink size={13} /></a> : <button type="button" onClick={() => void copyValue(item.value, item.key)} className="inline-flex shrink-0 items-center gap-1 text-[11px] font-bold text-[#087443] hover:text-[#065f46]">{copied === item.key ? <Check size={13} /> : <Copy size={13} />}{copied === item.key ? '已复制' : '复制'}</button>}
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="rounded-md border border-dashed border-[#cce8d6] bg-[#f0fdf4] px-4 py-5 text-center text-xs leading-5 text-[#087443]">客服入口已开启，但还没有配置具体联系方式。请管理员在后台“系统设置 → 客服入口”中填写。</div>
              )}
            </div>
          </section>
        </div>
      )}
    </>
  );
}
