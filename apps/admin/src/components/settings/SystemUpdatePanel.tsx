'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  ArrowUpCircle,
  CheckCircle2,
  CircleAlert,
  CircleOff,
  Clock3,
  ExternalLink,
  GitCommitHorizontal,
  Loader2,
  RefreshCw,
  Rocket,
  ShieldCheck,
} from 'lucide-react';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { portalApi, type SystemBuildVersion, type SystemUpdateInfo, type SystemUpdateState } from '@/lib/admin-api';
import { reloadForBuild } from '@/lib/build-version';

const activeStatuses = new Set<SystemUpdateState['status']>([
  'queued', 'waiting_idle', 'checking', 'pulling', 'backing_up', 'updating', 'rolling_back',
]);

const statusContent: Record<SystemUpdateState['status'], { label: string; description: string; tone: string }> = {
  unconfigured: { label: '未配置', description: '服务器尚未启用手动更新 worker', tone: 'bg-zinc-100 text-zinc-600' },
  idle: { label: '就绪', description: '等待管理员检查并确认新版本', tone: 'bg-zinc-100 text-zinc-600' },
  queued: { label: '已提交', description: '更新请求已进入服务器队列', tone: 'bg-cyan-50 text-cyan-700' },
  waiting_idle: { label: '等待空闲', description: '正在等待系统任务全部完成后再更新', tone: 'bg-blue-50 text-blue-700' },
  checking: { label: '校验版本', description: '正在核对 GitHub Actions 构建信息', tone: 'bg-cyan-50 text-cyan-700' },
  pulling: { label: '拉取镜像', description: '正在下载同版本的前台、后台和 API 镜像', tone: 'bg-cyan-50 text-cyan-700' },
  backing_up: { label: '准备更新', description: '旧版更新流程正在完成准备工作', tone: 'bg-amber-50 text-amber-700' },
  updating: { label: '更新服务', description: '正在替换应用容器并执行健康检查', tone: 'bg-amber-50 text-amber-700' },
  rolling_back: { label: '正在回退', description: '健康检查未通过，正在恢复上一版本', tone: 'bg-red-50 text-red-700' },
  success: { label: '更新成功', description: '新版本已通过全部健康检查', tone: 'bg-emerald-50 text-emerald-700' },
  failed: { label: '更新失败', description: '应用已恢复上一版本，数据库容器未变更', tone: 'bg-red-50 text-red-700' },
};

function formatDate(value?: string) {
  if (!value) return '暂无记录';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(date);
}

function shortCommit(value?: string) {
  return value && value !== 'local' ? value.slice(0, 8) : 'local';
}

function VersionColumn({ label, version, latest }: { label: string; version: SystemBuildVersion; latest?: boolean }) {
  return (
    <div className="min-w-0 px-5 py-4">
      <div className="flex items-center justify-between gap-3">
        <span className="text-[10px] font-bold text-zinc-400">{label}</span>
        {latest && <span className="status-pill success">Actions 已通过</span>}
      </div>
      <strong className="mt-2 block font-mono text-lg text-[#17201B]">{version.version || '未知版本'}</strong>
      <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-[10px] text-zinc-500">
        <span className="inline-flex items-center gap-1.5"><GitCommitHorizontal className="h-3.5 w-3.5" />{shortCommit(version.commit)}</span>
        <span className="inline-flex items-center gap-1.5"><Clock3 className="h-3.5 w-3.5" />{formatDate(version.publishedAt)}</span>
        {version.url && <a href={version.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 font-semibold text-[#047857] hover:underline">构建记录<ExternalLink className="h-3 w-3" /></a>}
      </div>
    </div>
  );
}

export function SystemUpdatePanel() {
  const [info, setInfo] = useState<SystemUpdateInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [checking, setChecking] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmMode, setConfirmMode] = useState<'idle' | 'wait' | 'force'>('idle');
  const previousStatus = useRef<SystemUpdateState['status'] | null>(null);
  const hasLoaded = useRef(false);

  const load = useCallback(async (refresh = false, silent = false) => {
    if (refresh) setChecking(true);
    if (!silent && !hasLoaded.current) setLoading(true);
    try {
      const response = await portalApi.systemUpdate(refresh);
      setInfo(response.data);
    } catch (error) {
      if (!silent) toast.error(error instanceof Error ? error.message : '版本信息加载失败');
    } finally {
      hasLoaded.current = true;
      setLoading(false);
      setChecking(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const active = info ? activeStatuses.has(info.state.status) : false;
  useEffect(() => {
    if (!active) return;
    const timer = window.setInterval(() => void load(false, true), 3000);
    return () => window.clearInterval(timer);
  }, [active, load]);

  const updateStatus = info?.state.status;
  const currentVersion = info?.current.version || '';
  useEffect(() => {
    const status = updateStatus;
    if (!status || previousStatus.current === status) return;
    if (status === 'success' && previousStatus.current && activeStatuses.has(previousStatus.current)) {
      toast.success('系统已更新，正在载入新版本');
      window.setTimeout(() => reloadForBuild(currentVersion), 1_000);
    }
    if (status === 'failed' && previousStatus.current && activeStatuses.has(previousStatus.current)) toast.error('更新失败，服务器已恢复上一版本');
    previousStatus.current = status;
  }, [currentVersion, updateStatus]);

  const startUpdate = async (force = false) => {
    setSubmitting(true);
    try {
      const response = await portalApi.startSystemUpdate(force);
      setInfo(response.data);
      toast.success(force ? '强制更新请求已提交' : '更新请求已提交，将在任务空闲后执行');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '更新请求提交失败');
    } finally {
      setSubmitting(false);
    }
  };

  const state = info?.state || { status: 'idle' as const };
  const stateMeta = statusContent[state.status] || statusContent.idle;
  const updateButtonLabel = active || submitting
    ? '正在更新'
    : info?.updateAvailable
      ? `空闲后更新到 ${info.latest.version}`
      : '已是最新版本';
  const pendingTaskCount = Number(info?.pendingTaskCount ?? info?.state.pendingTaskCount ?? 0);
  const waitingForIdle = state.status === 'waiting_idle';
  const canStartNormalUpdate = Boolean(info?.canUpdate && !submitting && !active);
  const canStartForceUpdate = Boolean(!submitting && ((info?.canUpdate && !active) || waitingForIdle));
  const openConfirm = (mode: 'wait' | 'force') => {
    setConfirmMode(mode);
    setConfirmOpen(true);
  };
  const confirmDescription = confirmMode === 'force'
    ? `将不等待当前 ${pendingTaskCount.toLocaleString('zh-CN')} 个运行中任务，直接更新 API、管理后台和客户前台。数据库容器保持不变，健康检查失败会恢复上一版本应用镜像。`
    : `普通更新会先等待系统内排队、处理中任务全部完成；当前检测到 ${pendingTaskCount.toLocaleString('zh-CN')} 个运行中任务。任务清零后会自动更新 API、管理后台和客户前台。`;

  return (
    <>
      <section className="section-panel overflow-hidden">
        <div className="section-head">
          <div className="flex min-w-0 items-center gap-2.5">
            <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-[#EAF8EF] text-[#047857]"><Rocket className="h-4 w-4" /></span>
            <span className="min-w-0"><strong className="block">系统更新</strong><small className="block truncate">检测 GitHub Actions 已通过的正式构建</small></span>
          </div>
          <button type="button" onClick={() => void load(true)} disabled={checking || active} className="btn" title="重新检查版本">
            <RefreshCw className={`h-3.5 w-3.5 ${checking ? 'animate-spin' : ''}`} />检查更新
          </button>
        </div>

        {loading ? (
          <div className="grid min-h-36 place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div>
        ) : info ? (
          <>
            <div className="grid grid-cols-1 divide-y divide-[#EDF0EE] md:grid-cols-2 md:divide-x md:divide-y-0">
              <VersionColumn label="当前运行版本" version={info.current} />
              <VersionColumn label="最新可用版本" version={info.latest} latest={Boolean(info.latest.runId)} />
            </div>
            <div className="flex flex-col gap-3 border-t border-[#EDF0EE] bg-[#FAFBFA] px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex min-w-0 items-start gap-3">
                <span className={`mt-0.5 inline-flex min-h-6 shrink-0 items-center rounded-full px-2 text-[10px] font-bold ${stateMeta.tone}`}>{stateMeta.label}</span>
                <span className="min-w-0">
                  <strong className="block text-[11px] text-[#3F4943]">{info.checkError || state.message || stateMeta.description}</strong>
                  <small className="mt-0.5 block truncate text-[10px] text-zinc-400">上次检查：{formatDate(info.checkedAt)} · 当前任务：{pendingTaskCount.toLocaleString('zh-CN')} 个</small>
                </span>
              </div>
              <div className="flex shrink-0 flex-wrap items-center gap-2">
                <button type="button" onClick={() => openConfirm('wait')} disabled={!canStartNormalUpdate} className="btn primary shrink-0">
                  {active || submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : info.updateAvailable ? <ArrowUpCircle className="h-4 w-4" /> : <CheckCircle2 className="h-4 w-4" />}
                  {updateButtonLabel}
                </button>
                <button type="button" onClick={() => openConfirm('force')} disabled={!canStartForceUpdate} className="btn danger shrink-0" title="跳过任务等待，直接更新应用容器">
                  {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <CircleOff className="h-4 w-4" />}
                  强制更新
                </button>
              </div>
            </div>
            {!info.configured && <div className="flex items-center gap-2 border-t border-amber-200 bg-amber-50 px-5 py-3 text-[11px] text-amber-800"><CircleAlert className="h-4 w-4 shrink-0" />宿主机更新服务尚未安装，版本检测可用，但更新按钮暂不可用。</div>}
            {info.configured && info.updateAvailable && !active && <div className="flex items-center gap-2 border-t border-emerald-100 bg-emerald-50/70 px-5 py-3 text-[11px] text-emerald-800"><ShieldCheck className="h-4 w-4 shrink-0" />普通更新会等待任务清零后执行；强制更新会跳过等待。仅替换应用容器，不备份或重建 PostgreSQL。</div>}
          </>
        ) : (
          <div className="empty-row">版本信息暂不可用</div>
        )}
      </section>

      <ConfirmDialog
        isOpen={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={() => void startUpdate(confirmMode === 'force')}
        title={`${confirmMode === 'force' ? '强制更新到' : '空闲后更新到'} ${info?.latest.version || '最新版本'}`}
        description={confirmDescription}
        confirmText={confirmMode === 'force' ? '强制更新' : '等待空闲后更新'}
        type={confirmMode === 'force' ? 'danger' : 'warning'}
      />
    </>
  );
}
