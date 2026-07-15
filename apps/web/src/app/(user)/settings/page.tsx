'use client';

import { useEffect, useState } from 'react';
import {
  BadgeCheck,
  Eye,
  EyeOff,
  KeyRound,
  LoaderCircle,
  LockKeyhole,
  MailCheck,
  ShieldCheck,
  UserRound,
} from 'lucide-react';
import { toast } from 'sonner';
import { PageHeader } from '@/components/common/PageHeader';
import { APIError, getSession, portalApi, refreshSession, type PortalUser } from '@/lib/portal-api';
import { formatDate } from '@/lib/common/utils';

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '密码更新失败，请稍后重试';
}

type PasswordFieldProps = {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  autoComplete: string;
  placeholder: string;
};

function PasswordField({ id, label, value, onChange, autoComplete, placeholder }: PasswordFieldProps) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <div className="relative">
        <input
          id={id}
          className="pr-10"
          type={visible ? 'text' : 'password'}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          autoComplete={autoComplete}
          placeholder={placeholder}
          required
        />
        <button
          type="button"
          className="absolute right-1 top-1/2 grid h-8 w-8 -translate-y-1/2 place-items-center rounded-md border-0 bg-transparent text-zinc-400 hover:bg-zinc-100 hover:text-zinc-700"
          onClick={() => setVisible((current) => !current)}
          title={visible ? '隐藏密码' : '显示密码'}
          aria-label={visible ? '隐藏密码' : '显示密码'}
        >
          {visible ? <EyeOff size={14} /> : <Eye size={14} />}
        </button>
      </div>
    </div>
  );
}

export default function SettingsPage() {
  const [user, setUser] = useState<PortalUser | null>(null);
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const current = getSession();
      if (!current) {
        setError('登录状态已失效，请重新登录');
        setLoading(false);
        return;
      }
      setUser(current);
      void refreshSession(current)
        .then(setUser)
        .catch((loadError) => setError(errorMessage(loadError)))
        .finally(() => setLoading(false));
    }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  const updatePassword = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!user) return;
    if (newPassword.length < 8) {
      toast.error('新密码至少需要 8 个字符');
      return;
    }
    if (newPassword.length > 72) {
      toast.error('新密码最多 72 个字符');
      return;
    }
    if (newPassword !== confirmPassword) {
      toast.error('两次输入的新密码不一致');
      return;
    }
    if (newPassword === oldPassword) {
      toast.error('新密码不能与当前密码相同');
      return;
    }

    setSaving(true);
    try {
      await portalApi.changePassword(user, oldPassword, newPassword);
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
      toast.success('密码已更新');
    } catch (saveError) {
      toast.error(errorMessage(saveError));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="page-stack">
      <PageHeader title="账户设置" description="查看账户身份并更新登录密码" />

      {error && <div className="notice" role="alert">{error}</div>}

      <section className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <form className="section-panel" onSubmit={(event) => void updatePassword(event)}>
          <div className="section-head">
            <div className="flex items-center gap-2"><LockKeyhole size={16} className="text-[#087443]" /><strong>修改密码</strong></div>
            <span className="status-pill active">账户安全</span>
          </div>
          <div className="section-body">
            <div className="grid max-w-xl gap-4">
              <PasswordField
                id="current-password"
                label="当前密码"
                value={oldPassword}
                onChange={setOldPassword}
                autoComplete="current-password"
                placeholder="输入当前登录密码"
              />
              <PasswordField
                id="new-password"
                label="新密码"
                value={newPassword}
                onChange={setNewPassword}
                autoComplete="new-password"
                placeholder="8-72 个字符"
              />
              <PasswordField
                id="confirm-password"
                label="确认新密码"
                value={confirmPassword}
                onChange={setConfirmPassword}
                autoComplete="new-password"
                placeholder="再次输入新密码"
              />
              <div className="notice flex items-start gap-2">
                <ShieldCheck size={15} className="mt-0.5 shrink-0" />
                <span>密码长度为 8-72 个字符。更新成功后，请在后续登录中使用新密码。</span>
              </div>
              <div className="flex justify-end">
                <button
                  className="btn primary"
                  type="submit"
                  disabled={saving || !oldPassword || !newPassword || !confirmPassword}
                >
                  {saving ? <LoaderCircle size={14} className="animate-spin" /> : <KeyRound size={14} />}
                  更新密码
                </button>
              </div>
            </div>
          </div>
        </form>

        <aside className="section-panel">
          <div className="section-head"><strong>账户信息</strong><UserRound size={16} className="text-zinc-400" /></div>
          <div className="section-body">
            {loading && !user ? (
              <div className="empty-row">正在读取账户信息...</div>
            ) : user ? (
              <div className="grid gap-4">
                <div className="flex items-center gap-3">
                  <span className="account-avatar h-10 w-10 text-sm">{user.email.slice(0, 1).toUpperCase()}</span>
                  <div className="min-w-0"><strong className="block truncate text-xs">{user.email}</strong><small className="text-[10px] text-zinc-500">{user.role === 'admin' ? '系统管理员' : 'API 客户'}</small></div>
                </div>
                <dl className="grid gap-3 border-y border-[#edf0ee] py-4 text-[10px]">
                  <div className="flex items-center justify-between gap-3"><dt className="text-zinc-500">账户 ID</dt><dd className="mono max-w-[190px] truncate" title={user.id}>{user.id}</dd></div>
                  <div className="flex items-center justify-between gap-3"><dt className="text-zinc-500">账户状态</dt><dd><span className={`status-pill ${user.status === 'active' ? 'active' : 'disabled'}`}>{user.status === 'active' ? '正常' : user.status}</span></dd></div>
                  <div className="flex items-center justify-between gap-3"><dt className="text-zinc-500">注册时间</dt><dd className="mono">{user.createdAt ? formatDate(user.createdAt, false) : '-'}</dd></div>
                </dl>
                <div className="grid gap-2 text-[10px] text-zinc-500">
                  <span className="flex items-center gap-2"><MailCheck size={13} className="text-blue-600" />登录邮箱：{user.emailVerifiedAt ? '已验证' : '未验证'}</span>
                  <span className="flex items-center gap-2"><BadgeCheck size={13} className="text-[#087443]" />API 权限与账户状态同步</span>
                </div>
              </div>
            ) : (
              <div className="empty-row">账户信息暂不可用</div>
            )}
          </div>
        </aside>
      </section>
    </div>
  );
}
