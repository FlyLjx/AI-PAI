'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Activity,
  Clock3,
  Eye,
  Globe2,
  Loader2,
  RefreshCw,
  ServerCrash,
  ShieldAlert,
  X,
} from 'lucide-react';
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { AppSelect } from '@/components/common/AppSelect';
import { DataTable } from '@/components/common/DataTable';
import { PageHeader } from '@/components/common/PageHeader';
import { StatBlock } from '@/components/common/StatBlock';
import {
  portalApi,
  type RequestMonitorLog,
  type RequestMonitorRange,
  type RequestMonitorSnapshot,
} from '@/lib/admin-api';
import { formatDate } from '@/lib/common/utils';

const EMPTY_SNAPSHOT: RequestMonitorSnapshot = {
  range: '24h',
  summary: { total: 0, successful: 0, clientErrors: 0, serverErrors: 0, errorRate: 0, averageDurationMs: 0, uniqueSources: 0 },
  trend: [],
  topEndpoints: [],
  topSources: [],
  items: [],
};

const METHOD_OPTIONS = [
  { value: 'all', label: '全部方法' },
  { value: 'GET', label: 'GET' },
  { value: 'POST', label: 'POST' },
  { value: 'PATCH', label: 'PATCH' },
  { value: 'PUT', label: 'PUT' },
  { value: 'DELETE', label: 'DELETE' },
];

const STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'success', label: '正常响应' },
  { value: 'client_error', label: '4xx 客户端错误' },
  { value: 'server_error', label: '5xx 服务端错误' },
];

function statusMeta(statusCode: number) {
  if (statusCode >= 500) return { label: String(statusCode), className: 'border-red-200 bg-red-50 text-red-700' };
  if (statusCode >= 400) return { label: String(statusCode), className: 'border-amber-200 bg-amber-50 text-amber-800' };
  if (statusCode >= 300) return { label: String(statusCode), className: 'border-blue-200 bg-blue-50 text-blue-700' };
  return { label: String(statusCode), className: 'border-emerald-200 bg-emerald-50 text-emerald-700' };
}

function methodClass(method: string) {
  if (method === 'POST') return 'border-blue-200 bg-blue-50 text-blue-700';
  if (method === 'PATCH' || method === 'PUT') return 'border-amber-200 bg-amber-50 text-amber-800';
  if (method === 'DELETE') return 'border-red-200 bg-red-50 text-red-700';
  return 'border-zinc-200 bg-zinc-50 text-zinc-600';
}

function durationLabel(value: number) {
  const milliseconds = Math.max(0, Number(value || 0));
  if (milliseconds < 1000) return `${Math.round(milliseconds)}ms`;
  return `${(milliseconds / 1000).toFixed(milliseconds < 10_000 ? 2 : 1)}s`;
}

function bytesLabel(value: number) {
  const bytes = Math.max(0, Number(value || 0));
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function sourceLabel(item: RequestMonitorLog) {
  return item.sourceHost || item.sourceIp || '未知来源';
}

function paramsText(value: unknown) {
  if (value === null || value === undefined) return '{}';
  if (typeof value === 'string') return value;
  return JSON.stringify(value, null, 2);
}

export default function RequestMonitorPage() {
  const [snapshot, setSnapshot] = useState<RequestMonitorSnapshot>(EMPTY_SNAPSHOT);
  const [range, setRange] = useState<RequestMonitorRange>('24h');
  const [method, setMethod] = useState('all');
  const [status, setStatus] = useState('all');
  const [search, setSearch] = useState('');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [selected, setSelected] = useState<RequestMonitorLog | null>(null);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setKeyword(search.trim());
      setPage(1);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [search]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await portalApi.requestMonitor({
        range,
        page,
        pageSize: 30,
        keyword: keyword || undefined,
        method: method === 'all' ? undefined : method,
        status: status === 'all' ? undefined : status,
      });
      setSnapshot(response.data);
      setTotal(response.pagination?.total || 0);
      setError('');
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '请求监控数据加载失败');
    } finally {
      setLoading(false);
    }
  }, [keyword, method, page, range, status]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const totalPages = Math.max(1, Math.ceil(total / 30));
  const maxEndpointCount = useMemo(() => Math.max(1, ...snapshot.topEndpoints.map((item) => item.count)), [snapshot.topEndpoints]);
  const maxSourceCount = useMemo(() => Math.max(1, ...snapshot.topSources.map((item) => item.count)), [snapshot.topSources]);

  return (
    <div className="space-y-5">
      <PageHeader title="请求监控" description="查看进入站点接口的请求来源、参数、频率、响应状态与耗时。">
        <button type="button" onClick={() => void load()} disabled={loading} className="inline-flex h-8 items-center gap-2 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold text-[#17201B] hover:border-[#12B76A] disabled:opacity-50">
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />刷新
        </button>
      </PageHeader>

      <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-[#DCE4DF] bg-white p-3">
        <div className="inline-flex rounded-md border border-[#DCE4DF] bg-[#F7F8F6] p-0.5" role="group" aria-label="监控时间范围">
          {([['1h', '1小时'], ['24h', '24小时'], ['7d', '7天'], ['30d', '30天']] as const).map(([value, label]) => (
            <button key={value} type="button" onClick={() => { setRange(value); setPage(1); }} aria-pressed={range === value} className={`h-7 min-w-14 rounded px-2 text-[11px] font-semibold ${range === value ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500 hover:text-zinc-800'}`}>{label}</button>
          ))}
        </div>
        <span className="text-[11px] text-zinc-400">只记录 API 请求，健康检查和监控查询已排除</span>
      </div>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700" role="alert"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
        <StatBlock title="请求总量" value={snapshot.summary.total.toLocaleString('zh-CN')} subtext={`正常 ${snapshot.summary.successful.toLocaleString('zh-CN')} 次`} icon={Activity} color="cyan" />
        <StatBlock title="请求错误" value={(snapshot.summary.clientErrors + snapshot.summary.serverErrors).toLocaleString('zh-CN')} subtext={`错误率 ${snapshot.summary.errorRate.toFixed(2)}%`} icon={ServerCrash} color={snapshot.summary.serverErrors ? 'amber' : 'neutral'} />
        <StatBlock title="平均耗时" value={durationLabel(snapshot.summary.averageDurationMs)} subtext="接口响应平均用时" icon={Clock3} color="green" />
        <StatBlock title="独立来源 IP" value={snapshot.summary.uniqueSources.toLocaleString('zh-CN')} subtext="当前筛选范围去重" icon={Globe2} color="neutral" />
      </div>

      <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
        <header className="flex flex-wrap items-center justify-between gap-3 border-b border-[#EDF0EE] px-4 py-3">
          <div><h2 className="text-sm font-semibold text-[#17201B]">请求趋势</h2><p className="mt-0.5 text-[11px] text-zinc-400">请求量、正常响应与错误响应</p></div>
          {loading && <Loader2 className="h-4 w-4 animate-spin text-[#12B76A]" aria-label="加载中" />}
        </header>
        <div className="h-[260px] p-4 pl-1">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={snapshot.trend} margin={{ top: 4, right: 12, bottom: 0, left: -10 }}>
              <CartesianGrid stroke="#EDF0EE" vertical={false} />
              <XAxis dataKey="time" tickFormatter={(value) => { const date = new Date(String(value)); return range === '30d' || range === '7d' ? `${date.getMonth() + 1}/${date.getDate()}` : `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`; }} tick={{ fontSize: 10, fill: '#8A938E' }} tickLine={false} axisLine={false} minTickGap={24} />
              <YAxis allowDecimals={false} tick={{ fontSize: 10, fill: '#8A938E' }} tickLine={false} axisLine={false} />
              <Tooltip formatter={(value, name) => [Number(value || 0).toLocaleString('zh-CN'), ({ total: '请求量', successful: '正常', errors: '错误' } as Record<string, string>)[String(name)] || String(name)]} labelFormatter={(label) => formatDate(String(label))} contentStyle={{ border: '1px solid #DCE4DF', borderRadius: 7, boxShadow: '0 8px 24px rgba(23,32,27,.08)', fontSize: 10 }} />
              <Line type="monotone" dataKey="total" stroke="#587FA3" strokeWidth={2} dot={false} activeDot={{ r: 3 }} />
              <Line type="monotone" dataKey="successful" stroke="#3F9274" strokeWidth={2} dot={false} activeDot={{ r: 3 }} />
              <Line type="monotone" dataKey="errors" stroke="#D06F69" strokeWidth={2} dot={false} activeDot={{ r: 3 }} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </section>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <FrequencyPanel title="接口调用频率" items={snapshot.topEndpoints} maxCount={maxEndpointCount} emptyText="暂无接口请求" />
        <FrequencyPanel title="来源调用频率" items={snapshot.topSources} maxCount={maxSourceCount} emptyText="暂无来源数据" />
      </div>

      <section>
        <div className="mb-3 flex items-center justify-between gap-3"><div><h2 className="text-sm font-semibold text-[#17201B]">请求明细</h2><p className="mt-0.5 text-[11px] text-zinc-400">参数已自动脱敏，图片与上传文件不保存原始内容</p></div><span className="text-[11px] text-zinc-400">{total.toLocaleString('zh-CN')} 条</span></div>
        <DataTable
          headers={[
            { key: 'time', label: '请求时间' },
            { key: 'method', label: '方法' },
            { key: 'path', label: '请求接口' },
            { key: 'source', label: '来源域名 / IP' },
            { key: 'status', label: '状态' },
            { key: 'duration', label: '耗时' },
            { key: 'action', label: '参数' },
          ]}
          data={snapshot.items}
          searchPlaceholder="搜索接口、域名、IP 或 User-Agent"
          searchValue={search}
          onSearchChange={setSearch}
          filterControls={<><AppSelect compact value={method} options={METHOD_OPTIONS} onValueChange={(value) => { setMethod(value); setPage(1); }} ariaLabel="请求方法" /><AppSelect compact value={status} options={STATUS_OPTIONS} onValueChange={(value) => { setStatus(value); setPage(1); }} ariaLabel="响应状态" /></>}
          currentPage={page}
          totalPages={totalPages}
          onPageChange={setPage}
          renderRow={(item) => { const responseStatus = statusMeta(item.statusCode); return (
            <tr key={item.id} className="hover:bg-[#FAFBFA]">
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{formatDate(item.createdAt)}</td>
              <td className="px-4 py-3"><span className={`rounded border px-1.5 py-0.5 font-mono text-[10px] font-semibold ${methodClass(item.method)}`}>{item.method}</span></td>
              <td className="max-w-[300px] px-4 py-3"><span className="block truncate font-mono text-[11px] font-semibold text-[#17201B]" title={item.path}>{item.path}</span></td>
              <td className="max-w-[240px] px-4 py-3"><span className="block truncate text-[11px] font-semibold" title={sourceLabel(item)}>{sourceLabel(item)}</span><small className="mt-0.5 block truncate font-mono text-[9px] text-zinc-400">{item.sourceIp || '-'}</small></td>
              <td className="px-4 py-3"><span className={`rounded border px-1.5 py-0.5 font-mono text-[10px] font-semibold ${responseStatus.className}`}>{responseStatus.label}</span></td>
              <td className="whitespace-nowrap px-4 py-3 font-mono text-[11px]">{durationLabel(item.durationMs)}<small className="ml-1 text-[9px] text-zinc-400">{bytesLabel(item.responseBytes)}</small></td>
              <td className="px-4 py-3"><button type="button" onClick={() => setSelected(item)} title="查看请求参数" className="grid h-7 w-7 place-items-center rounded border border-[#DCE4DF] text-zinc-500 hover:border-[#12B76A] hover:text-[#047857]"><Eye className="h-3.5 w-3.5" /></button></td>
            </tr>
          ); }}
          renderMobileItem={(item) => { const responseStatus = statusMeta(item.statusCode); return (
            <button key={item.id} type="button" onClick={() => setSelected(item)} className="w-full rounded-md border border-[#DCE4DF] bg-white p-3 text-left">
              <div className="flex items-start justify-between gap-3"><span className="min-w-0"><strong className="block truncate font-mono text-xs">{item.path}</strong><small className="mt-1 block truncate text-[10px] text-zinc-400">{sourceLabel(item)} · {item.sourceIp || '-'}</small></span><span className={`shrink-0 rounded border px-1.5 py-0.5 font-mono text-[10px] font-semibold ${responseStatus.className}`}>{responseStatus.label}</span></div>
              <div className="mt-3 flex items-center justify-between border-t border-[#EDF0EE] pt-2 text-[10px] text-zinc-500"><span className={`rounded border px-1.5 py-0.5 font-mono ${methodClass(item.method)}`}>{item.method}</span><span>{durationLabel(item.durationMs)} · {formatDate(item.createdAt)}</span></div>
            </button>
          ); }}
        />
      </section>

      {selected && <RequestDetail item={selected} onClose={() => setSelected(null)} />}
    </div>
  );
}

function FrequencyPanel({ title, items, maxCount, emptyText }: { title: string; items: RequestMonitorSnapshot['topEndpoints']; maxCount: number; emptyText: string }) {
  return (
    <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
      <header className="border-b border-[#EDF0EE] px-4 py-3"><h2 className="text-sm font-semibold text-[#17201B]">{title}</h2></header>
      <div className="divide-y divide-[#EDF0EE]">
        {items.map((item) => {
          const errorRate = item.count ? item.errors / item.count * 100 : 0;
          return <div key={item.name} className="px-4 py-3"><div className="flex items-center justify-between gap-3 text-[11px]"><span className="min-w-0 truncate font-mono font-semibold text-[#17201B]" title={item.name}>{item.name}</span><span className="shrink-0 font-mono text-zinc-500">{item.count.toLocaleString('zh-CN')} 次</span></div><div className="mt-2 h-1.5 overflow-hidden rounded-full bg-[#EDF0EE]"><div className="h-full rounded-full bg-[#587FA3]" style={{ width: `${Math.max(3, item.count / maxCount * 100)}%` }} /></div><div className="mt-1.5 flex items-center justify-between text-[9px] text-zinc-400"><span>错误 {item.errors} · {errorRate.toFixed(1)}%</span><span>平均 {durationLabel(item.averageDurationMs)}</span></div></div>;
        })}
        {!items.length && <div className="px-4 py-10 text-center text-xs text-zinc-400">{emptyText}</div>}
      </div>
    </section>
  );
}

function RequestDetail({ item, onClose }: { item: RequestMonitorLog; onClose: () => void }) {
  const responseStatus = statusMeta(item.statusCode);
  return (
    <div className="fixed inset-0 z-80 grid place-items-center bg-black/45 p-4" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <section className="flex max-h-[calc(100dvh-2rem)] w-full max-w-4xl flex-col overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl" role="dialog" aria-modal="true" aria-labelledby="request-detail-title">
        <header className="flex items-center justify-between gap-3 border-b border-[#EDF0EE] px-4 py-3"><div className="min-w-0"><h2 id="request-detail-title" className="truncate text-sm font-semibold text-[#17201B]">请求详情</h2><p className="mt-0.5 truncate font-mono text-[10px] text-zinc-400">{item.id}</p></div><button type="button" onClick={onClose} title="关闭" className="grid h-8 w-8 shrink-0 place-items-center rounded text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></header>
        <div className="overflow-y-auto p-4">
          <div className="grid grid-cols-2 gap-px overflow-hidden rounded-md border border-[#DCE4DF] bg-[#DCE4DF] lg:grid-cols-4">
            {[
              ['接口', `${item.method} ${item.path}`],
              ['响应', `HTTP ${item.statusCode}`],
              ['耗时 / 大小', `${durationLabel(item.durationMs)} / ${bytesLabel(item.responseBytes)}`],
              ['请求时间', formatDate(item.createdAt)],
              ['来源域名', item.sourceHost || '-'],
              ['来源 IP', item.sourceIp || '-'],
              ['Origin', item.origin || '-'],
              ['Referer', item.referer || '-'],
            ].map(([label, value]) => <div key={label} className="min-w-0 bg-white p-3"><span className="block text-[10px] font-semibold text-zinc-400">{label}</span><strong className="mt-1 block break-all text-[11px] text-[#17201B]">{value}</strong></div>)}
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-2"><span className={`rounded border px-2 py-0.5 font-mono text-[10px] font-semibold ${methodClass(item.method)}`}>{item.method}</span><span className={`rounded border px-2 py-0.5 font-mono text-[10px] font-semibold ${responseStatus.className}`}>HTTP {responseStatus.label}</span><span className="inline-flex items-center gap-1 text-[10px] text-zinc-400"><ShieldAlert className="h-3 w-3" />敏感字段、Cookie、密钥与图片内容已脱敏</span></div>
          <div className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
            <ParameterBlock title="Query 参数" value={item.queryParams} />
            <ParameterBlock title="Body 参数" value={item.bodyParams} />
          </div>
          <div className="mt-4 rounded-md border border-[#DCE4DF] bg-[#FAFBFA] p-3"><span className="block text-[10px] font-semibold text-zinc-400">User-Agent</span><p className="mt-1 break-all font-mono text-[10px] leading-5 text-zinc-600">{item.userAgent || '-'}</p></div>
        </div>
      </section>
    </div>
  );
}

function ParameterBlock({ title, value }: { title: string; value: unknown }) {
  return <section className="min-w-0 overflow-hidden rounded-md border border-[#DCE4DF]"><header className="border-b border-[#EDF0EE] bg-[#FAFBFA] px-3 py-2 text-[11px] font-semibold text-[#17201B]">{title}</header><pre className="max-h-[360px] overflow-auto whitespace-pre-wrap break-all p-3 font-mono text-[10px] leading-5 text-zinc-600">{paramsText(value)}</pre></section>;
}
