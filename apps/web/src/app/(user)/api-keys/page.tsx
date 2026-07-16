'use client';

import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';
import {
  Check,
  Copy,
  Crown,
  Eye,
  LoaderCircle,
  LockKeyhole,
  MailWarning,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  Trash2,
  WalletCards,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import {
  APIError,
  getSession,
  portalApi,
  type APIKey,
  type APIKeyBillingMode,
  type PortalUser,
  type SelectableAPIKeyBillingMode,
} from '@/lib/portal-api';
import { formatDate } from '@/lib/common/utils';

type CreatedCredential = {
  id: string;
  name: string;
  secret: string;
  billingMode?: APIKeyBillingMode | null;
};

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '操作失败，请稍后重试';
}

function maskedPrefix(key: APIKey): string {
  return `${key.keyPrefix || 'ap-key'}••••••••`;
}

function scrubKey(key: APIKey): APIKey {
  return { ...key, key: undefined, keyPlain: undefined };
}

function billingModeMeta(mode?: APIKeyBillingMode | null) {
  if (mode === 'subscription') {
    return {
      label: '订阅额度',
      className: 'border-[#fde68a] bg-[#fffbeb] text-[#92400e]',
    };
  }
  if (mode === 'balance') {
    return {
      label: '账户余额',
      className: 'border-[#bfdbfe] bg-[#eff6ff] text-[#1d4ed8]',
    };
  }
  return {
    label: '自动兼容',
    className: 'border-zinc-200 bg-zinc-50 text-zinc-600',
  };
}

function BillingModeBadge({ mode }: { mode?: APIKeyBillingMode | null }) {
  const meta = billingModeMeta(mode);
  return (
    <span className={`inline-flex h-6 items-center whitespace-nowrap rounded border px-2 text-[11px] font-semibold ${meta.className}`}>
      {meta.label}
    </span>
  );
}

export default function ApiKeysPage() {
  const [user, setUser] = useState<PortalUser | null>(null);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [pendingId, setPendingId] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [newBillingMode, setNewBillingMode] = useState<SelectableAPIKeyBillingMode>('balance');
  const [creating, setCreating] = useState(false);
  const [createdCredential, setCreatedCredential] = useState<CreatedCredential | null>(null);
  const [copied, setCopied] = useState(false);

  const [revealingId, setRevealingId] = useState('');
  const [revealedCredential, setRevealedCredential] = useState<CreatedCredential | null>(null);
  const [revealedCopied, setRevealedCopied] = useState(false);

  const [keyToDelete, setKeyToDelete] = useState<APIKey | null>(null);
  const canCreateKey = Boolean(user?.emailVerifiedAt);

  const loadKeys = useCallback(async () => {
    const current = getSession();
    if (!current) {
      setError('登录状态已失效，请重新登录');
      setLoading(false);
      return;
    }
    setUser(current);
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.listKeys(current);
      setApiKeys((response.data || []).map(scrubKey));
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadKeys(), 0);
    return () => window.clearTimeout(timer);
  }, [loadKeys]);

  const closeCreateModal = () => {
    setShowCreateModal(false);
    setNewKeyName('');
    setNewBillingMode('balance');
    setCreatedCredential(null);
    setCopied(false);
  };

  const openCreateModal = () => {
    if (!canCreateKey) {
      toast.error('请先完成邮箱验证后创建 API Key');
      return;
    }
    setShowCreateModal(true);
  };

  const handleCreate = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!user || !newKeyName.trim()) return;
    if (!user.emailVerifiedAt) {
      closeCreateModal();
      toast.error('请先完成邮箱验证后创建 API Key');
      return;
    }
    setCreating(true);
    try {
      const response = await portalApi.createKey(user, newKeyName.trim(), newBillingMode);
      const created = response.data;
      const secret = String(created.key || created.keyPlain || '');
      if (!secret) throw new Error('API Key 已创建，但服务端未返回明文凭证');

      setApiKeys((items) => [
        scrubKey(created),
        ...items.filter((item) => item.id !== created.id),
      ]);
      setCreatedCredential({ id: created.id, name: created.name, secret, billingMode: newBillingMode });
      setNewKeyName('');
      toast.success('API Key 创建成功');
    } catch (createError) {
      toast.error(errorMessage(createError));
    } finally {
      setCreating(false);
    }
  };

  const copySecret = async () => {
    if (!createdCredential) return;
    try {
      await navigator.clipboard.writeText(createdCredential.secret);
      setCopied(true);
      toast.success('API Key 已复制');
    } catch {
      toast.error('复制失败，请手动选择密钥');
    }
  };

  const closeRevealModal = () => {
    setRevealedCredential(null);
    setRevealedCopied(false);
  };

  const revealKey = async (key: APIKey) => {
    if (!user || revealingId) return;
    setRevealingId(key.id);
    try {
      const response = await portalApi.revealKey(user, key.id);
      const secret = String(response.data.key || '');
      if (!secret) throw new Error('服务端未返回 API Key 明文');
      setRevealedCredential({ id: key.id, name: key.name, secret });
      setRevealedCopied(false);
    } catch (revealError) {
      toast.error(errorMessage(revealError));
    } finally {
      setRevealingId('');
    }
  };

  const copyRevealedSecret = async () => {
    if (!revealedCredential) return;
    try {
      await navigator.clipboard.writeText(revealedCredential.secret);
      setRevealedCopied(true);
      toast.success('API Key 已复制');
    } catch {
      toast.error('复制失败，请手动选择密钥');
    }
  };

  const toggleKey = async (key: APIKey) => {
    if (!user) return;
    const nextStatus = key.status === 'active' ? 'disabled' : 'active';
    setPendingId(key.id);
    try {
      const response = await portalApi.updateKey(user, key.id, nextStatus);
      setApiKeys((items) => items.map((item) => (
        item.id === key.id ? scrubKey(response.data) : item
      )));
      toast.success(nextStatus === 'active' ? 'API Key 已启用' : 'API Key 已停用');
    } catch (toggleError) {
      toast.error(errorMessage(toggleError));
    } finally {
      setPendingId('');
    }
  };

  const deleteKey = async () => {
    if (!user || !keyToDelete) return;
    const id = keyToDelete.id;
    setPendingId(id);
    try {
      await portalApi.deleteKey(user, id);
      setApiKeys((items) => items.filter((item) => item.id !== id));
      setKeyToDelete(null);
      toast.success('API Key 已删除');
    } catch (deleteError) {
      toast.error(errorMessage(deleteError));
    } finally {
      setPendingId('');
    }
  };

  const actions = (key: APIKey) => (
    <div className="action-row justify-end">
      <button
        className="btn icon"
        type="button"
        title="查看完整 API Key"
        aria-label={`查看 ${key.name} 的完整 API Key`}
        onClick={() => void revealKey(key)}
        disabled={Boolean(revealingId) || pendingId === key.id}
      >
        {revealingId === key.id
          ? <LoaderCircle size={14} className="animate-spin" />
          : <Eye size={14} />}
      </button>
      <button
        className="btn icon"
        type="button"
        title={key.status === 'active' ? '停用 API Key' : '启用 API Key'}
        aria-label={key.status === 'active' ? '停用 API Key' : '启用 API Key'}
        onClick={() => void toggleKey(key)}
        disabled={pendingId === key.id}
      >
        {pendingId === key.id
          ? <LoaderCircle size={14} className="animate-spin" />
          : key.status === 'active' ? <PowerOff size={14} /> : <Power size={14} />}
      </button>
      <button
        className="btn icon danger"
        type="button"
        title="删除 API Key"
        aria-label="删除 API Key"
        onClick={() => setKeyToDelete(key)}
        disabled={pendingId === key.id}
      >
        <Trash2 size={14} />
      </button>
    </div>
  );

  const headers = [
    { key: 'name', label: '名称' },
    { key: 'credential', label: '凭证前缀' },
    { key: 'billingMode', label: '计费方式' },
    { key: 'status', label: '状态' },
    { key: 'usage', label: '调用 / 成功' },
    { key: 'lastUsed', label: '最后调用' },
    { key: 'createdAt', label: '创建时间' },
    { key: 'actions', label: '操作', className: 'text-right' },
  ];

  return (
    <div className="page-stack">
      <PageHeader title="API Key" description="管理 OpenAI 兼容接口的访问凭证">
        <button className="btn" type="button" onClick={() => void loadKeys()} disabled={loading}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />刷新
        </button>
        <button className={`btn ${canCreateKey ? 'primary' : ''}`} type="button" onClick={openCreateModal} disabled={!canCreateKey} title={canCreateKey ? '创建 API Key' : '请先验证邮箱'}>
          <Plus size={14} />创建 Key
        </button>
      </PageHeader>

      {error && <div className="notice" role="alert">{error}</div>}
      {user && !canCreateKey && (
        <div className="flex flex-col items-start gap-3 rounded-[7px] border border-[#fed7aa] bg-[#fff7ed] px-3 py-2.5 text-[12px] leading-5 text-[#9a4a08] sm:flex-row sm:items-center" role="status">
          <MailWarning size={16} className="shrink-0" />
          <span className="min-w-0 flex-1">邮箱尚未验证，当前账户可以登录和查看用量，但暂不能创建 API Key。</span>
          <Link className="btn shrink-0" href="/settings">验证邮箱</Link>
        </div>
      )}

      {loading && apiKeys.length === 0 ? (
        <div className="section-panel empty-row">正在读取 API Key...</div>
      ) : (
        <DataTable
          headers={headers}
          data={apiKeys}
          renderRow={(key) => (
            <tr key={key.id}>
              <td className="px-4 py-3 font-semibold">{key.name}</td>
              <td className="px-4 py-3 mono text-[12px]">{maskedPrefix(key)}</td>
              <td className="px-4 py-3"><BillingModeBadge mode={key.billingMode} /></td>
              <td className="px-4 py-3">
                <StatusBadge status={key.status === 'active' ? 'active' : 'disabled'} />
              </td>
              <td className="px-4 py-3 mono">{Number(key.requestCount || 0)} / {Number(key.successCount || 0)}</td>
              <td className="px-4 py-3 mono text-zinc-500">{key.lastUsedAt ? formatDate(key.lastUsedAt) : '从未调用'}</td>
              <td className="px-4 py-3 mono text-zinc-500">{formatDate(key.createdAt)}</td>
              <td className="px-4 py-3">{actions(key)}</td>
            </tr>
          )}
          renderMobileItem={(key) => (
            <article key={key.id} className="section-panel p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <strong className="block truncate text-sm">{key.name}</strong>
                  <code className="mt-1 block truncate text-[11px] text-zinc-500">{maskedPrefix(key)}</code>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1.5">
                  <StatusBadge status={key.status === 'active' ? 'active' : 'disabled'} />
                  <BillingModeBadge mode={key.billingMode} />
                </div>
              </div>
              <div className="mt-3 grid grid-cols-2 gap-2 border-y border-[#edf0ee] py-3 text-[11px] text-zinc-500">
                <span>调用 <strong className="mono text-[#17201b]">{Number(key.requestCount || 0)}</strong></span>
                <span>成功 <strong className="mono text-[#17201b]">{Number(key.successCount || 0)}</strong></span>
                <span className="col-span-2">最后调用：{key.lastUsedAt ? formatDate(key.lastUsedAt) : '从未调用'}</span>
              </div>
              <div className="mt-3 flex justify-end">{actions(key)}</div>
            </article>
          )}
          emptyState={(
            <EmptyState
              title="还没有 API Key"
              description={canCreateKey ? '创建凭证后即可调用 OpenAI 兼容 API。' : '完成邮箱验证后即可创建访问凭证。'}
              icon={LockKeyhole}
              action={canCreateKey
                ? <button className="btn primary" type="button" onClick={openCreateModal}><Plus size={14} />创建 Key</button>
                : <Link className="btn primary" href="/settings"><MailWarning size={14} />验证邮箱</Link>}
            />
          )}
        />
      )}

      {showCreateModal && (
        <div className="modal-backdrop" role="presentation">
          <section className="modal-panel" role="dialog" aria-modal="true" aria-labelledby="create-key-title">
            <div className="modal-title">
              <strong id="create-key-title">{createdCredential ? '保存 API Key' : '创建 API Key'}</strong>
              <button type="button" onClick={closeCreateModal} title="关闭" aria-label="关闭"><X size={17} /></button>
            </div>
            {createdCredential ? (
              <div className="modal-content space-y-4">
                <div className="notice flex gap-2">
                  <Check size={16} className="mt-0.5 shrink-0" />
                  <span>密钥已创建。你可以立即复制，也可稍后在 Key 列表中再次查看。</span>
                </div>
                <div className="flex items-center justify-between border-y border-[#edf0ee] py-2.5 text-[12px] text-zinc-500">
                  <span>固定计费方式</span>
                  <BillingModeBadge mode={createdCredential.billingMode} />
                </div>
                <div className="field">
                  <label htmlFor="created-api-key">{createdCredential.name}</label>
                  <div className="flex items-stretch gap-2">
                    <input
                      id="created-api-key"
                      className="mono min-w-0 flex-1"
                      type="text"
                      value={createdCredential.secret}
                      readOnly
                      onFocus={(event) => event.currentTarget.select()}
                    />
                    <button className="btn icon self-start" type="button" onClick={() => void copySecret()} title="复制 API Key" aria-label="复制 API Key">
                      {copied ? <Check size={15} /> : <Copy size={15} />}
                    </button>
                  </div>
                </div>
                <div className="flex justify-end">
                  <button className="btn primary" type="button" onClick={closeCreateModal}>我已保存</button>
                </div>
              </div>
            ) : (
              <form className="modal-content space-y-4" onSubmit={(event) => void handleCreate(event)}>
                <div className="field">
                  <label htmlFor="key-name">Key 名称</label>
                  <input
                    id="key-name"
                    value={newKeyName}
                    onChange={(event) => setNewKeyName(event.target.value)}
                    maxLength={50}
                    placeholder="例如：生产环境"
                    autoFocus
                    required
                  />
                </div>
                <fieldset className="space-y-2">
                  <legend className="text-[12px] font-semibold text-[#17201b]">计费方式</legend>
                  <div className="grid grid-cols-2 gap-1 rounded-[7px] border border-[#dce4df] bg-[#f6f8f6] p-1">
                    <button
                      type="button"
                      aria-pressed={newBillingMode === 'balance'}
                      onClick={() => setNewBillingMode('balance')}
                      className={`flex h-10 items-center justify-center gap-1.5 rounded-[5px] border text-[12px] font-semibold transition-colors ${newBillingMode === 'balance' ? 'border-[#86efac] bg-[#f0fdf4] text-[#047857]' : 'border-transparent text-zinc-500 hover:bg-white'}`}
                    >
                      <WalletCards size={15} />账户余额
                    </button>
                    <button
                      type="button"
                      aria-pressed={newBillingMode === 'subscription'}
                      onClick={() => setNewBillingMode('subscription')}
                      className={`flex h-10 items-center justify-center gap-1.5 rounded-[5px] border text-[12px] font-semibold transition-colors ${newBillingMode === 'subscription' ? 'border-[#86efac] bg-[#f0fdf4] text-[#047857]' : 'border-transparent text-zinc-500 hover:bg-white'}`}
                    >
                      <Crown size={15} />订阅额度
                    </button>
                  </div>
                  <p className="text-[11px] leading-5 text-zinc-500" aria-live="polite">
                    {newBillingMode === 'balance'
                      ? '按模型价格从账户余额扣除，余额不足时该 Key 停止调用。'
                      : '仅扣当前订阅图片额度，订阅到期或额度不足时该 Key 停止调用。'}
                    <span className="block">计费方式创建后固定，如需切换请新建 Key。</span>
                  </p>
                </fieldset>
                <div className="flex justify-end gap-2">
                  <button className="btn" type="button" onClick={closeCreateModal}>取消</button>
                  <button className="btn primary" type="submit" disabled={creating || !newKeyName.trim()}>
                    {creating && <LoaderCircle size={14} className="animate-spin" />}
                    创建
                  </button>
                </div>
              </form>
            )}
          </section>
        </div>
      )}

      {revealedCredential && (
        <div className="modal-backdrop" role="presentation">
          <section className="modal-panel" role="dialog" aria-modal="true" aria-labelledby="reveal-key-title">
            <div className="modal-title">
              <strong id="reveal-key-title">查看 API Key</strong>
              <button type="button" onClick={closeRevealModal} title="关闭" aria-label="关闭"><X size={17} /></button>
            </div>
            <div className="modal-content space-y-4">
              <div className="notice flex gap-2">
                <LockKeyhole size={16} className="mt-0.5 shrink-0" />
                <span>完整密钥属于敏感凭证，请仅在受信任的环境中查看和复制。</span>
              </div>
              <div className="field">
                <label htmlFor="revealed-api-key">{revealedCredential.name}</label>
                <div className="flex items-stretch gap-2">
                  <input
                    id="revealed-api-key"
                    className="mono min-w-0 flex-1"
                    type="text"
                    value={revealedCredential.secret}
                    readOnly
                    onFocus={(event) => event.currentTarget.select()}
                  />
                  <button className="btn icon self-start" type="button" onClick={() => void copyRevealedSecret()} title="复制 API Key" aria-label="复制 API Key">
                    {revealedCopied ? <Check size={15} /> : <Copy size={15} />}
                  </button>
                </div>
              </div>
              <div className="flex justify-end">
                <button className="btn primary" type="button" onClick={closeRevealModal}>关闭</button>
              </div>
            </div>
          </section>
        </div>
      )}

      <ConfirmDialog
        isOpen={Boolean(keyToDelete)}
        onClose={() => setKeyToDelete(null)}
        onConfirm={() => void deleteKey()}
        title="删除 API Key"
        description={`删除“${keyToDelete?.name || ''}”后，使用该凭证的请求会立即失效，此操作不可撤销。`}
        confirmText="确认删除"
        type="danger"
      />
    </div>
  );
}
