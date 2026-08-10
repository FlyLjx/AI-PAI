'use client';

import { useEffect, useState } from 'react';
import {
  BadgeCheck,
  Eye,
  EyeOff,
  ExternalLink,
  KeyRound,
  LoaderCircle,
  LockKeyhole,
  Mail,
  MailCheck,
  Send,
  ShieldCheck,
} from 'lucide-react';
import { toast } from 'sonner';
import { APIError, getSession, portalApi, refreshSession, type EmailChangeRequest, type PortalUser } from '@/lib/portal-api';
import { formatDate } from '@/lib/common/utils';
import styles from './page.module.css';

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '操作失败，请稍后重试';
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
    <div className={styles.field}>
      <label htmlFor={id}>{label}</label>
      <div className={styles.passwordControl}>
        <input
          id={id}
          type={visible ? 'text' : 'password'}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          autoComplete={autoComplete}
          placeholder={placeholder}
          required
        />
        <button
          type="button"
          className={styles.visibilityButton}
          onClick={() => setVisible((current) => !current)}
          title={visible ? '隐藏密码' : '显示密码'}
          aria-label={visible ? '隐藏密码' : '显示密码'}
        >
          {visible ? <EyeOff size={17} /> : <Eye size={17} />}
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
  const [newEmail, setNewEmail] = useState('');
  const [emailPassword, setEmailPassword] = useState('');
  const [emailRequest, setEmailRequest] = useState<EmailChangeRequest | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [emailSaving, setEmailSaving] = useState(false);
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

  const requestEmailChange = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!user) return;
    const normalizedEmail = newEmail.trim().toLowerCase();
    if (normalizedEmail === user.email.trim().toLowerCase()) {
      toast.error('新邮箱不能与当前邮箱相同');
      return;
    }
    setEmailSaving(true);
    setEmailRequest(null);
    try {
      const response = await portalApi.requestEmailChange(user, emailPassword, normalizedEmail);
      setEmailRequest(response.data);
      setEmailPassword('');
      toast.success(response.data.sent ? '验证邮件已发送' : '邮箱验证链接已生成');
    } catch (saveError) {
      toast.error(errorMessage(saveError));
    } finally {
      setEmailSaving(false);
    }
  };

  return (
    <div className={styles.page}>
      {error && <div className={styles.errorNotice} role="alert">{error}</div>}

      <section className={styles.accountSummary} aria-label="账户摘要">
        {loading && !user ? (
          <div className={styles.summaryLoading}>
            <LoaderCircle size={18} className={styles.spinner} />
            正在读取账户信息...
          </div>
        ) : user ? (
          <>
            <div className={styles.identity}>
              <span className={styles.avatar}>{user.email.slice(0, 1).toUpperCase()}</span>
              <div className={styles.identityText}>
                <strong title={user.email}>{user.email}</strong>
                <span>{user.role === 'admin' ? '系统管理员' : 'API 客户'}</span>
                <span className={`${styles.statusBadge} ${user.status === 'active' ? styles.successBadge : styles.neutralBadge}`}>
                  {user.status === 'active' ? '账户正常' : user.status}
                </span>
              </div>
            </div>

            <dl className={styles.summaryDetails}>
              <div className={styles.summaryItem}>
                <dt>账户 ID</dt>
                <dd className={styles.mono} title={user.id}>{user.id}</dd>
              </div>
              <div className={styles.summaryItem}>
                <dt>邮箱验证</dt>
                <dd className={user.emailVerifiedAt ? styles.successText : styles.warningText}>
                  {user.emailVerifiedAt ? '已验证' : '未验证'}
                </dd>
              </div>
              <div className={styles.summaryItem}>
                <dt>注册时间</dt>
                <dd>{user.createdAt ? formatDate(user.createdAt, false) : '-'}</dd>
              </div>
              <div className={styles.summaryItem}>
                <dt>API 权限</dt>
                <dd className={user.status === 'active' ? styles.successText : styles.warningText}>
                  {user.status === 'active' ? '正常' : '受限'}
                </dd>
              </div>
            </dl>
          </>
        ) : (
          <div className={styles.summaryLoading}>账户信息暂不可用</div>
        )}
      </section>

      <section className={styles.formsGrid}>
        <form className={styles.formCard} onSubmit={(event) => void requestEmailChange(event)}>
          <div className={styles.cardHeader}>
            <span className={styles.headerIcon}><Mail size={18} /></span>
            <strong>修改登录邮箱</strong>
            <span className={`${styles.statusBadge} ${styles.successBadge}`}>验证后生效</span>
          </div>

          <div className={styles.formContent}>
            <div className={styles.field}>
              <label htmlFor="current-email">当前邮箱</label>
              <input id="current-email" type="email" value={user?.email || ''} disabled />
            </div>
            <div className={styles.field}>
              <label htmlFor="new-email">新邮箱</label>
              <input
                id="new-email"
                type="email"
                required
                maxLength={120}
                autoComplete="email"
                value={newEmail}
                onChange={(event) => setNewEmail(event.target.value)}
                placeholder="请输入新邮箱"
              />
            </div>
            <PasswordField
              id="email-current-password"
              label="当前密码"
              value={emailPassword}
              onChange={setEmailPassword}
              autoComplete="current-password"
              placeholder="请输入当前密码"
            />

            <div className={styles.hint}>
              <MailCheck size={15} />
              <span>验证邮件将发送至新邮箱，确认前原邮箱仍然有效。</span>
            </div>

            {emailRequest && (
              <div className={styles.requestResult} role="status">
                <div><BadgeCheck size={15} /><span>{emailRequest.message || `验证邮件已发送到 ${emailRequest.email}`}</span></div>
                {emailRequest.verificationUrl && (
                  <a href={emailRequest.verificationUrl} target="_blank" rel="noreferrer">
                    打开本地验证链接 <ExternalLink size={12} />
                  </a>
                )}
              </div>
            )}

            <div className={styles.actions}>
              <button className={styles.primaryButton} type="submit" disabled={emailSaving || !user || !newEmail.trim() || !emailPassword}>
                {emailSaving ? <LoaderCircle size={15} className={styles.spinner} /> : <Send size={15} />}
                发送验证邮件
              </button>
            </div>
          </div>
        </form>

        <form className={styles.formCard} onSubmit={(event) => void updatePassword(event)}>
          <div className={styles.cardHeader}>
            <span className={styles.headerIcon}><LockKeyhole size={18} /></span>
            <strong>修改密码</strong>
            <span className={`${styles.statusBadge} ${styles.successBadge}`}>账户安全</span>
          </div>

          <div className={styles.formContent}>
            <PasswordField
              id="current-password"
              label="当前密码"
              value={oldPassword}
              onChange={setOldPassword}
              autoComplete="current-password"
              placeholder="请输入当前密码"
            />
            <PasswordField
              id="new-password"
              label="新密码"
              value={newPassword}
              onChange={setNewPassword}
              autoComplete="new-password"
              placeholder="请输入新密码"
            />
            <PasswordField
              id="confirm-password"
              label="确认新密码"
              value={confirmPassword}
              onChange={setConfirmPassword}
              autoComplete="new-password"
              placeholder="请再次输入新密码"
            />

            <div className={styles.hint}>
              <ShieldCheck size={15} />
              <span>密码长度为 8-72 个字符</span>
            </div>

            <div className={styles.actions}>
              <button className={styles.primaryButton} type="submit" disabled={saving || !oldPassword || !newPassword || !confirmPassword}>
                {saving ? <LoaderCircle size={15} className={styles.spinner} /> : <KeyRound size={15} />}
                更新密码
              </button>
            </div>
          </div>
        </form>
      </section>
    </div>
  );
}
