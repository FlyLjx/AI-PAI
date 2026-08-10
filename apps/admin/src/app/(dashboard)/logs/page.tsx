'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Activity, AlertTriangle, CalendarClock, Clipboard, Database, FileText, HardDrive, Loader2, RefreshCw, Save, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect } from '@/components/common/AppSelect';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { EmptyState } from '@/components/common/EmptyState';
import { AdminMetricCard } from '@/components/common/AdminMetricCard';
import { PageHeader } from '@/components/common/PageHeader';
import { type SystemLogDetail, type SystemLogFile, portalApi } from '@/lib/admin-api';
import { formatDate } from '@/lib/common/utils';

const CATEGORY_OPTIONS = [
  { value: 'all', label: '全部分类' },
  { value: 'system', label: '系统' },
  { value: 'api', label: 'API' },
  { value: 'generation', label: '调用' },
  { value: 'error', label: '错误' },
] as const;

function bytes(value: number) {
  const size = Number(value || 0);
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(2)} MB`;
}

function normalizeRetentionDays(value: unknown) {
  const days = Number(value);
  return Number.isSafeInteger(days) && days >= 1 && days <= 3650 ? days : 30;
}

function categoryView(category: string) {
  if (category === 'error') return { label: '错误', className: 'border-red-200 bg-red-50 text-red-700' };
  if (category === 'api') return { label: 'API', className: 'border-blue-200 bg-blue-50 text-blue-700' };
  if (category === 'generation') return { label: '调用', className: 'border-amber-200 bg-amber-50 text-amber-700' };
  return { label: '系统', className: 'border-zinc-200 bg-zinc-50 text-zinc-600' };
}

function isErrorLogLine(line: string) {
  return /\[(?:ERROR|FATAL|PANIC)\]|(?:^|\s)(?:ERROR|FATAL|PANIC)(?:\s|:)|\b(?:level|severity)\s*[=:]\s*["']?(?:error|fatal|panic)\b|\b(?:panic:|fatal:|runtime error:|uncaught exception|uncaught error|unhandled rejection|stack trace|traceback \(|exception:)|"status"\s*:\s*"failed"|"error(?:Message)?"\s*:\s*"[^"\s]/i.test(line);
}

export default function AdminLogsPage() {
  const [files, setFiles] = useState<SystemLogFile[]>([]);
  const [detail, setDetail] = useState<SystemLogDetail | null>(null);
  const [selectedName, setSelectedName] = useState('');
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('all');
  const [deleteCandidate, setDeleteCandidate] = useState<SystemLogFile | null>(null);
  const [cleanupEnabled, setCleanupEnabled] = useState(false);
  const [retentionDays, setRetentionDays] = useState(30);
  const [cleanupSettingsLoading, setCleanupSettingsLoading] = useState(true);
  const [cleanupSettingsSaving, setCleanupSettingsSaving] = useState(false);
  const [cleanupSettingsError, setCleanupSettingsError] = useState('');
  const contentRef = useRef<HTMLPreElement>(null);

  const loadDetail = useCallback(async (name: string) => {
    if (!name) {
      setDetail(null);
      return;
    }
    setDetailLoading(true);
    try {
      const response = await portalApi.systemLogDetail(name);
      setDetail(response.data);
      setSelectedName(name);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '日志内容加载失败');
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const loadFiles = useCallback(async (keepSelection = true) => {
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.logs();
      setFiles(response.data);
      const nextName = keepSelection && response.data.some((file) => file.name === selectedName)
        ? selectedName
        : response.data[0]?.name || '';
      if (nextName) await loadDetail(nextName);
      else {
        setSelectedName('');
        setDetail(null);
      }
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '系统日志加载失败');
    } finally {
      setLoading(false);
    }
  }, [loadDetail, selectedName]);

  const loadCleanupSettings = useCallback(async () => {
    setCleanupSettingsLoading(true);
    setCleanupSettingsError('');
    try {
      const response = await portalApi.settings();
      setCleanupEnabled(response.data.systemLogAutoCleanupEnabled === true);
      setRetentionDays(normalizeRetentionDays(response.data.systemLogRetentionDays));
    } catch (requestError) {
      setCleanupSettingsError(requestError instanceof Error ? requestError.message : '自动清理设置加载失败');
    } finally {
      setCleanupSettingsLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void Promise.all([loadFiles(false), loadCleanupSettings()]), 0);
    return () => window.clearTimeout(timer);
    // Only the initial file selection should run on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loadCleanupSettings]);

  useEffect(() => {
    if (detailLoading || !detail?.content) return;
    const frame = window.requestAnimationFrame(() => {
      if (contentRef.current) contentRef.current.scrollTop = contentRef.current.scrollHeight;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [detail?.content, detail?.name, detailLoading]);

  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    return files.filter((file) => {
      const matchesKeyword = !keyword || file.name.toLowerCase().includes(keyword);
      return matchesKeyword && (categoryFilter === 'all' || file.category === categoryFilter);
    });
  }, [categoryFilter, files, search]);


  const summary = useMemo(() => ({
    count: files.length,
    totalSize: files.reduce((sum, file) => sum + Number(file.size || 0), 0),
    errors: files.filter((file) => file.category === 'error').length,
    api: files.filter((file) => file.category === 'api' || file.category === 'generation').length,
  }), [files]);

  const detailLines = useMemo(() => detail?.content.split('\n') || [], [detail?.content]);
  const cleanupControlsDisabled = cleanupSettingsLoading || cleanupSettingsSaving || Boolean(cleanupSettingsError);

  const copyContent = async () => {
    if (!detail?.content) return toast.error('当前日志没有内容');
    try {
      await navigator.clipboard.writeText(detail.content);
      toast.success('日志内容已复制');
    } catch {
      toast.error('复制失败，请手动选择日志内容');
    }
  };

  const deleteLog = async () => {
    if (!deleteCandidate) return;
    try {
      await portalApi.deleteSystemLog(deleteCandidate.name);
      toast.success('日志已清理');
      setDeleteCandidate(null);
      await loadFiles(false);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '日志清理失败');
    }
  };

  const saveCleanupSettings = async () => {
    if (!Number.isSafeInteger(retentionDays) || retentionDays < 1 || retentionDays > 3650) {
      toast.error('日志保留天数必须是 1 到 3650 的整数');
      return;
    }
    setCleanupSettingsSaving(true);
    setCleanupSettingsError('');
    try {
      const response = await portalApi.updateSettings({
        systemLogAutoCleanupEnabled: cleanupEnabled,
        systemLogRetentionDays: retentionDays,
      });
      const values = response.data as Record<string, unknown>;
      setCleanupEnabled(values.systemLogAutoCleanupEnabled === true);
      setRetentionDays(normalizeRetentionDays(values.systemLogRetentionDays));
      toast.success(cleanupEnabled ? `自动清理已启用，日志保留 ${retentionDays} 天` : '日志自动清理已关闭');
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : '自动清理设置保存失败';
      setCleanupSettingsError(message);
      toast.error(message);
    } finally {
      setCleanupSettingsSaving(false);
    }
  };

  return (
    <div className="space-y-5">
      <PageHeader title="系统日志" description="查看 Go 服务运行日志、API 调用日志和错误文件。">
        <button type="button" onClick={() => void Promise.all([loadFiles(), loadCleanupSettings()])} disabled={loading || detailLoading || cleanupSettingsLoading} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading || detailLoading || cleanupSettingsLoading ? 'animate-spin' : ''}`} />刷新</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          { title: '日志文件', value: summary.count, note: '当前可读取', icon: FileText, tone: 'blue' as const },
          { title: '占用空间', value: bytes(summary.totalSize), note: '日志目录合计', icon: HardDrive, tone: 'neutral' as const },
          { title: '错误日志', value: summary.errors, note: '错误分类文件', icon: AlertTriangle, tone: 'red' as const },
          { title: 'API / 调用', value: summary.api, note: '中转请求日志', icon: Activity, tone: 'green' as const },
        ].map((metric) => <AdminMetricCard key={metric.title} title={metric.title} value={metric.value} note={metric.note} icon={metric.icon} tone={metric.tone} />)}
      </div>

      <section aria-label="日志自动清理" className="flex flex-col gap-3 border-y border-[#DCE4DF] bg-white px-4 py-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-emerald-50 text-[#047857]"><CalendarClock className="h-4 w-4" /></span>
          <span className="min-w-0"><strong className="block text-xs">日志自动清理</strong><small className="mt-0.5 block text-[10px] text-zinc-400">{cleanupSettingsLoading ? '正在读取清理策略' : cleanupEnabled ? `每小时检查，保留最近 ${retentionDays} 天` : '当前关闭，历史日志持续保留'}</small></span>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <label className="inline-flex h-8 items-center gap-2 rounded-md border border-[#DCE4DF] bg-[#FAFBFA] px-3 text-[11px] font-semibold text-zinc-600"><input type="checkbox" checked={cleanupEnabled} onChange={(event) => setCleanupEnabled(event.target.checked)} disabled={cleanupControlsDisabled} className="h-3.5 w-3.5 accent-[#047857]" />启用</label>
          <label className={`flex h-8 items-center overflow-hidden rounded-md border border-[#DCE4DF] bg-white ${cleanupEnabled && !cleanupSettingsError ? '' : 'opacity-50'}`}><span className="border-r border-[#DCE4DF] bg-[#FAFBFA] px-2.5 text-[10px] text-zinc-500">保留</span><input type="number" min={1} max={3650} step={1} value={retentionDays} onChange={(event) => setRetentionDays(Number(event.target.value))} disabled={!cleanupEnabled || cleanupControlsDisabled} aria-label="日志保留天数" className="h-full w-20 border-0 px-2 text-center font-mono text-xs outline-none disabled:bg-zinc-50" /><span className="border-l border-[#DCE4DF] bg-[#FAFBFA] px-2.5 text-[10px] text-zinc-500">天</span></label>
          <button type="button" onClick={() => void saveCleanupSettings()} disabled={cleanupControlsDisabled} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-[11px] font-semibold text-white hover:bg-[#036B4F] disabled:opacity-50">{cleanupSettingsSaving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}保存</button>
        </div>
        {cleanupSettingsError && <p className="text-[10px] text-red-600 sm:basis-full">{cleanupSettingsError}</p>}
      </section>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void loadFiles(false)} className="font-semibold underline">重试</button></div>}

      {loading && !files.length ? (
        <div className="grid min-h-[320px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : !files.length ? (
        <EmptyState title="暂无系统日志" description="Go 服务产生日志文件后会显示在这里。" icon={Database} />
      ) : (
        <div className="grid min-h-[560px] grid-cols-1 overflow-hidden rounded-md border border-[#DCE4DF] bg-white lg:grid-cols-[280px_minmax(0,1fr)]">
          <aside className="border-b border-[#DCE4DF] bg-[#FAFBFA] lg:border-b-0 lg:border-r">
            <div className="space-y-2 border-b border-[#DCE4DF] p-3">
              <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索日志文件" className="h-8 w-full rounded-md border border-[#DCE4DF] bg-white px-3 text-xs outline-none focus:border-[#12B76A]" />
              <AppSelect value={categoryFilter} options={CATEGORY_OPTIONS} onValueChange={setCategoryFilter} compact className="w-full" ariaLabel="筛选日志分类" />
            </div>
            <div className="max-h-[360px] space-y-1 overflow-y-auto p-2 lg:max-h-[500px]">
              {filtered.map((file) => { const category = categoryView(file.category); const selected = file.name === selectedName; return (
                <button key={file.name} type="button" onClick={() => void loadDetail(file.name)} className={`w-full rounded-md border px-3 py-2.5 text-left transition-colors ${selected ? 'border-[#86EFAC] bg-[#F0FDF4]' : 'border-transparent hover:border-[#DCE4DF] hover:bg-white'}`}>
                  <span className="flex items-start justify-between gap-2"><span className="min-w-0"><strong className={`block truncate font-mono text-[11px] ${file.category === 'error' ? 'text-red-700' : 'text-[#17201B]'}`}>{file.name}</strong><small className="mt-1 block text-[10px] text-zinc-400">{formatDate(file.updatedAt)}</small></span><span className={`shrink-0 rounded border px-1.5 py-0.5 text-[10px] font-semibold ${category.className}`}>{category.label}</span></span>
                  <span className="mt-1.5 block text-[10px] text-zinc-400">{bytes(file.size)}</span>
                </button>
              ); })}
              {!filtered.length && <p className="py-8 text-center text-[11px] text-zinc-400">无匹配日志</p>}
            </div>
          </aside>

          <section className="flex min-w-0 flex-col">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[#DCE4DF] px-4 py-3">
              <div className="min-w-0"><h2 className="truncate font-mono text-xs font-semibold">{detail?.name || selectedName || '选择日志文件'}</h2>{detail && <p className="mt-0.5 text-[10px] text-zinc-400">{bytes(detail.size)} · 偏移 {detail.offset.toLocaleString('zh-CN')}{detail.truncated ? ' · 仅显示末尾内容' : ''}</p>}</div>
              <div className="flex items-center gap-1.5">
                <button type="button" onClick={() => void loadDetail(selectedName)} disabled={!selectedName || detailLoading} title="刷新内容" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white text-zinc-600 hover:border-[#12B76A] disabled:opacity-40"><RefreshCw className={`h-3.5 w-3.5 ${detailLoading ? 'animate-spin' : ''}`} /></button>
                <button type="button" onClick={() => void copyContent()} disabled={!detail?.content} title="复制日志" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white text-zinc-600 hover:border-[#12B76A] disabled:opacity-40"><Clipboard className="h-3.5 w-3.5" /></button>
                <button type="button" onClick={() => { const file = files.find((item) => item.name === selectedName); if (file) setDeleteCandidate(file); }} disabled={!selectedName} title="清理日志" className="grid h-8 w-8 place-items-center rounded-md border border-red-200 bg-red-50 text-red-600 hover:bg-red-100 disabled:opacity-40"><Trash2 className="h-3.5 w-3.5" /></button>
              </div>
            </div>
            <div className="relative min-h-[420px] flex-1 bg-[#111815]">
              {detailLoading ? <div className="absolute inset-0 grid place-items-center bg-[#111815]/80"><Loader2 className="h-6 w-6 animate-spin text-[#86EFAC]" /></div> : detail?.content ? <pre ref={contentRef} className="h-full max-h-[620px] overflow-auto whitespace-pre-wrap break-words p-4 font-mono text-[12px] leading-5 text-[#D5E4DB]">{detailLines.map((line, index) => <span key={`${index}-${line.slice(0, 24)}`} className={`block min-h-5 ${isErrorLogLine(line) ? 'text-[#FF8A80]' : ''}`}>{line || ' '}</span>)}</pre> : <div className="grid min-h-[420px] place-items-center text-center text-zinc-500"><span><FileText className="mx-auto mb-2 h-8 w-8" /><small>当前日志为空</small></span></div>}
            </div>
          </section>
        </div>
      )}

      <ConfirmDialog isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)} onConfirm={() => void deleteLog()} title="清理系统日志" description={`确定清理 ${deleteCandidate?.name || '该日志'} 吗？当前正在写入的日志会被截断清空，其余日志文件会被删除。`} confirmText="清理" type="danger" />
    </div>
  );
}
