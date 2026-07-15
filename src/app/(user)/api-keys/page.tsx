'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  Check,
  Copy,
  LoaderCircle,
  LockKeyhole,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  Trash2,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import { APIError, getSession, portalApi, type APIKey, type PortalUser } from '@/lib/portal-api';
import { formatDate } from '@/lib/common/utils';

type CreatedCredential = {
  id: string;
  name: string;
  secret: string;
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

export default function ApiKeysPage() {
  const [user, setUser] = useState<PortalUser | null>(null);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [pendingId, setPendingId] = useState('');

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [creating, setCreating] = useState(false);
  const [createdCredential, setCreatedCredential] = useState<CreatedCredential | null>(null);
  const [copied, setCopied] = useState(false);

  const [keyToDelete, setKeyToDelete] = useState<APIKey | null>(null);

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
    setCreatedCredential(null);
    setCopied(false);
  };

  const handleCreate = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!user || !newKeyName.trim()) return;
    setCreating(true);
    try {
      const response = await portalApi.createKey(user, newKeyName.trim());
      const created = response.data;
      const secret = String(created.key || created.keyPlain || '');
      if (!secret) throw new Error('API Key 已创建，但服务端未返回明文凭证');

      setApiKeys((items) => [
        scrubKey(created),
        ...items.filter((item) => item.id !== created.id),
      ]);
      setCreatedCredential({ id: created.id, name: created.name, secret });
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
        <button className="btn primary" type="button" onClick={() => setShowCreateModal(true)}>
          <Plus size={14} />创建 Key
        </button>
      </PageHeader>

      {error && <div className="notice" role="alert">{error}</div>}

      {loading && apiKeys.length === 0 ? (
        <div className="section-panel empty-row">正在读取 API Key...</div>
      ) : (
        <DataTable
          headers={headers}
          data={apiKeys}
          renderRow={(key) => (
            <tr key={key.id}>
              <td className="px-4 py-3 font-semibold">{key.name}</td>
              <td className="px-4 py-3 mono text-[11px]">{maskedPrefix(key)}</td>
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
                  <code className="mt-1 block truncate text-[10px] text-zinc-500">{maskedPrefix(key)}</code>
                </div>
                <StatusBadge status={key.status === 'active' ? 'active' : 'disabled'} />
              </div>
              <div className="mt-3 grid grid-cols-2 gap-2 border-y border-[#edf0ee] py-3 text-[10px] text-zinc-500">
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
              description="创建凭证后即可调用 OpenAI 兼容 API。"
              icon={LockKeyhole}
              action={<button className="btn primary" type="button" onClick={() => setShowCreateModal(true)}><Plus size={14} />创建 Key</button>}
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
                  <span>密钥明文仅在本次创建后展示。关闭窗口前请妥善保存。</span>
                </div>
                <div className="field">
                  <label htmlFor="created-api-key">{createdCredential.name}</label>
                  <div className="flex items-stretch gap-2">
                    <textarea
                      id="created-api-key"
                      className="mono min-h-[84px] flex-1 break-all"
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
