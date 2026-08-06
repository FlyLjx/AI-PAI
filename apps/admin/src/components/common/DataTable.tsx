'use client';

import React from 'react';
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronLeft, ChevronRight, Search } from 'lucide-react';
import { AppSelect } from './AppSelect';

type PaginationItem = number | 'ellipsis-left' | 'ellipsis-right';

function paginationItems(currentPage: number, totalPages: number): PaginationItem[] {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, index) => index + 1);

  const pages = new Set([1, totalPages, currentPage - 1, currentPage, currentPage + 1]);
  if (currentPage <= 4) [2, 3, 4, 5].forEach((page) => pages.add(page));
  if (currentPage >= totalPages - 3) {
    [totalPages - 4, totalPages - 3, totalPages - 2, totalPages - 1].forEach((page) => pages.add(page));
  }

  const sorted = [...pages].filter((page) => page >= 1 && page <= totalPages).sort((a, b) => a - b);
  const items: PaginationItem[] = [];
  sorted.forEach((page, index) => {
    const previous = sorted[index - 1];
    if (previous && page - previous > 1) items.push(previous === 1 ? 'ellipsis-left' : 'ellipsis-right');
    items.push(page);
  });
  return items;
}

export type SortDirection = 'asc' | 'desc';

export type SortState = {
  key: string;
  direction: SortDirection;
};

export interface TableHeader<T> {
  key: string;
  label: string;
  className?: string;
  sortable?: boolean;
  sortValue?: (item: T) => unknown;
}

interface DataTableProps<T> {
  headers: TableHeader<T>[];
  data: T[];
  renderRow: (item: T, index: number) => React.ReactNode;
  renderMobileItem: (item: T, index: number) => React.ReactNode;

  // Search & Filter
  searchPlaceholder?: string;
  searchValue?: string;
  onSearchChange?: (val: string) => void;

  // Custom headers slot
  filterControls?: React.ReactNode;

  // Pagination
  currentPage?: number;
  totalPages?: number;
  onPageChange?: (page: number) => void;
  pageSize?: number;
  clientSidePagination?: boolean;

  // Sorting
  sortKey?: string;
  sortDirection?: SortDirection;
  onSort?: (key: string, direction: SortDirection) => void;
  serverSideSorting?: boolean;

  emptyState?: React.ReactNode;
}

function fallbackSortValue<T>(item: T, key: string): unknown {
  if (item && typeof item === 'object') return (item as Record<string, unknown>)[key];
  return undefined;
}

function isEmptySortValue(value: unknown) {
  return value === null || value === undefined || value === '' || (typeof value === 'number' && Number.isNaN(value));
}

function compareSortValues(left: unknown, right: unknown): number {
  const leftEmpty = isEmptySortValue(left);
  const rightEmpty = isEmptySortValue(right);
  if (leftEmpty && rightEmpty) return 0;
  if (leftEmpty) return 1;
  if (rightEmpty) return -1;

  if (typeof left === 'number' && typeof right === 'number') {
    if (Number.isNaN(left) && Number.isNaN(right)) return 0;
    if (Number.isNaN(left)) return 1;
    if (Number.isNaN(right)) return -1;
    return left - right;
  }

  if (typeof left === 'boolean' && typeof right === 'boolean') {
    return Number(left) - Number(right);
  }

  return String(left).localeCompare(String(right), 'zh-CN', { numeric: true, sensitivity: 'base' });
}

export function sortItems<T>(items: T[], header: TableHeader<T>, direction: SortDirection): T[] {
  const multiplier = direction === 'asc' ? 1 : -1;
  return items
    .map((item, index) => ({ item, index }))
    .sort((left, right) => {
      const leftValue = header.sortValue ? header.sortValue(left.item) : fallbackSortValue(left.item, header.key);
      const rightValue = header.sortValue ? header.sortValue(right.item) : fallbackSortValue(right.item, header.key);
      const result = compareSortValues(leftValue, rightValue);
      if (isEmptySortValue(leftValue) || isEmptySortValue(rightValue)) {
        return result === 0 ? left.index - right.index : result;
      }
      return result === 0 ? left.index - right.index : result * multiplier;
    })
    .map(({ item }) => item);
}

export function SortableHeader<T>({
  header,
  sortState,
  onSort,
  className = '',
}: {
  header: TableHeader<T>;
  sortState?: SortState | null;
  onSort?: (key: string) => void;
  className?: string;
}) {
  const sortable = header.sortable !== false;
  const active = sortState?.key === header.key;
  const Icon = active
    ? sortState?.direction === 'asc' ? ArrowUp : ArrowDown
    : ArrowUpDown;

  return (
    <th className={`px-4 py-2.5 text-xs font-semibold text-[#17201B]/60 tracking-wider ${header.className || ''} ${className}`}>
      {sortable ? (
        <button
          type="button"
          onClick={() => onSort?.(header.key)}
          className={`inline-flex min-h-6 items-center gap-1 text-left transition-colors ${active ? 'text-[#047857]' : 'hover:text-[#047857]'}`}
          aria-label={`${header.label}排序`}
          aria-pressed={active}
          title={`按${header.label}排序`}
        >
          <span>{header.label}</span>
          <Icon className={`h-3.5 w-3.5 shrink-0 ${active ? 'opacity-100' : 'opacity-45'}`} aria-hidden="true" />
        </button>
      ) : header.label}
    </th>
  );
}

export function DataTable<T>({
  headers,
  data,
  renderRow,
  renderMobileItem,
  searchPlaceholder = '搜索...',
  searchValue,
  onSearchChange,
  filterControls,
  currentPage,
  totalPages,
  onPageChange,
  pageSize,
  clientSidePagination = false,
  sortKey,
  sortDirection,
  onSort,
  serverSideSorting = false,
  emptyState
}: DataTableProps<T>) {
  const [internalSort, setInternalSort] = React.useState<SortState | null>(null);
  const controlledSort = sortKey ? { key: sortKey, direction: sortDirection || 'asc' } : null;
  const sortState = controlledSort || internalSort;
  const activeHeader = sortState ? headers.find((header) => header.key === sortState.key) : undefined;
  const sortedData = activeHeader && !serverSideSorting ? sortItems(data, activeHeader, sortState?.direction || 'asc') : data;
  const pageOffset = clientSidePagination && currentPage && pageSize ? (currentPage - 1) * pageSize : 0;
  const renderedData = clientSidePagination && currentPage && pageSize
    ? sortedData.slice(pageOffset, pageOffset + pageSize)
    : sortedData;

  const handleSort = (key: string) => {
    const nextDirection: SortDirection = sortState?.key === key && sortState.direction === 'asc' ? 'desc' : 'asc';
    if (onSort) {
      onSort(key, nextDirection);
    } else {
      setInternalSort({ key, direction: nextDirection });
    }
  };

  const showPagination = !!(currentPage && totalPages && totalPages > 1);
  const pageOptions = showPagination && totalPages
    ? Array.from({ length: totalPages }, (_, index) => ({ value: String(index + 1), label: `第 ${index + 1} 页` }))
    : [];
  const visiblePages = showPagination && currentPage && totalPages ? paginationItems(currentPage, totalPages) : [];

  return (
    <div className="space-y-4">
      {/* Search & Filter bar */}
      {(onSearchChange || filterControls) && (
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 bg-white p-3 border border-[#DCE4DF] rounded-md">
          {onSearchChange && (
            <div className="relative flex-1 max-w-sm">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-[#17201B]/40" />
              <input
                type="text"
                placeholder={searchPlaceholder}
                value={searchValue}
                onChange={(e) => onSearchChange(e.target.value)}
                className="w-full pl-9 pr-3 py-1.5 border border-[#DCE4DF] rounded-md text-xs bg-white placeholder-[#17201B]/40 focus:outline-none focus:ring-1 focus:ring-[#12B76A] focus:border-[#12B76A] font-sans"
              />
            </div>
          )}
          {filterControls && (
            <div className="flex flex-wrap items-center gap-2.5 sm:justify-end">
              {filterControls}
            </div>
          )}
        </div>
      )}

      {/* Main Table view */}
      {renderedData.length === 0 ? (
        emptyState || (
          <div className="text-center p-8 bg-white border border-[#DCE4DF] rounded-md text-xs text-[#17201B]/50 font-sans">
            无匹配数据
          </div>
        )
      ) : (
        <>
          {/* Desktop Table View */}
          <div className="hidden md:block overflow-x-auto bg-white border border-[#DCE4DF] rounded-md shadow-sm">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="bg-[#F6F8F6] border-b border-[#DCE4DF]">
                  {headers.map((header) => (
                    <SortableHeader
                      key={header.key}
                      header={header}
                      sortState={sortState}
                      onSort={handleSort}
                    />
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-[#DCE4DF] text-xs text-[#17201B]">
                {renderedData.map((item, idx) => renderRow(item, pageOffset + idx))}
              </tbody>
            </table>
          </div>

          {/* Mobile Collapsed Cards View */}
          <div className="block md:hidden space-y-3.5">
            {renderedData.map((item, idx) => renderMobileItem(item, pageOffset + idx))}
          </div>
        </>
      )}

      {/* Pagination Footer */}
      {showPagination && currentPage && totalPages && onPageChange && (
        <div className="flex flex-col gap-3 bg-white px-4 py-3 border border-[#DCE4DF] rounded-md text-xs shadow-sm sm:flex-row sm:items-center sm:justify-between">
          <div className="text-[#17201B]/60 font-sans">
            第 <span className="font-mono font-semibold text-[#17201B]">{currentPage}</span> 页，共{' '}
            <span className="font-mono font-semibold text-[#17201B]">{totalPages}</span> 页
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <AppSelect
              compact
              value={String(currentPage)}
              options={pageOptions}
              onValueChange={(value) => onPageChange(Number(value))}
              ariaLabel="选择页码"
            />
            <button
              type="button"
              title="上一页"
              aria-label="上一页"
              onClick={() => onPageChange(currentPage - 1)}
              disabled={currentPage === 1}
              className="grid h-8 w-8 place-items-center border border-[#DCE4DF] rounded-md bg-white text-[#17201B] hover:bg-zinc-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <div className="hidden items-center gap-1 sm:flex" aria-label="分页页码">
              {visiblePages.map((item) => typeof item === 'number' ? (
                <button
                  key={item}
                  type="button"
                  onClick={() => onPageChange(item)}
                  aria-label={`第 ${item} 页`}
                  aria-current={item === currentPage ? 'page' : undefined}
                  className={`grid h-8 min-w-8 place-items-center rounded-md border px-2 font-mono text-[11px] font-semibold transition-colors ${item === currentPage ? 'border-[#12B76A] bg-[#F0FDF4] text-[#047857]' : 'border-[#DCE4DF] bg-white text-zinc-600 hover:border-[#86EFAC] hover:text-[#047857]'}`}
                >
                  {item}
                </button>
              ) : (
                <span key={item} className="grid h-8 w-5 place-items-center text-zinc-400" aria-hidden="true">...</span>
              ))}
            </div>
            <button
              type="button"
              title="下一页"
              aria-label="下一页"
              onClick={() => onPageChange(currentPage + 1)}
              disabled={currentPage === totalPages}
              className="grid h-8 w-8 place-items-center border border-[#DCE4DF] rounded-md bg-white text-[#17201B] hover:bg-zinc-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
