'use client';

import React from 'react';
import { ChevronLeft, ChevronRight, Search } from 'lucide-react';

interface Header {
  key: string;
  label: string;
  className?: string;
}
type PaginationItem = number | 'ellipsis';

interface DataTableProps<T> {
  headers: Header[];
  data: T[];
  renderRow: (item: T, index: number) => React.ReactNode;
  renderMobileItem?: (item: T, index: number) => React.ReactNode;

  // Search & Filter
  searchPlaceholder?: string;
  searchValue?: string;
  onSearchChange?: (val: string) => void;
  filterControls?: React.ReactNode;

  // Pagination
  currentPage?: number;
  totalPages?: number;
  totalItems?: number;
  onPageChange?: (page: number) => void;
  paginationDisabled?: boolean;

  emptyState?: React.ReactNode;
  loading?: boolean;
  loadingState?: React.ReactNode;

  // Layout hooks used by pages that need their established table styles.
  className?: string;
  tableWrapClassName?: string;
  tableClassName?: string;
  mobileListClassName?: string;
  embedded?: boolean;
}

function pageItems(currentPage: number, totalPages: number): PaginationItem[] {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, index) => index + 1);

  const items: PaginationItem[] = [1];
  const start = Math.max(2, currentPage - 1);
  const end = Math.min(totalPages - 1, currentPage + 1);

  if (start > 2) items.push('ellipsis');
  for (let page = start; page <= end; page += 1) items.push(page);
  if (end < totalPages - 1) items.push('ellipsis');
  items.push(totalPages);
  return items;
}

interface TablePaginationProps {
  currentPage: number;
  totalPages: number;
  totalItems: number;
  onPageChange: (page: number) => void;
  disabled?: boolean;
}

function TablePagination({ currentPage, totalPages, totalItems, onPageChange, disabled = false }: TablePaginationProps) {
  const items = pageItems(currentPage, totalPages);

  return (
    <footer className="table-pagination" aria-label="分页">
      <span className="table-pagination-summary">
        第 <strong>{currentPage}</strong> / <strong>{totalPages}</strong> 页 · 共 {totalItems.toLocaleString('zh-CN')} 条
      </span>
      <div className="table-pagination-controls">
        <button
          type="button"
          className="table-pagination-button"
          onClick={() => onPageChange(Math.max(1, currentPage - 1))}
          disabled={disabled || currentPage <= 1}
          aria-label="上一页"
        >
          <ChevronLeft size={14} />
        </button>
        <div className="table-pagination-pages" aria-label="页码">
          {items.map((item, index) => item === 'ellipsis' ? (
            <span key={`ellipsis-${index}`} aria-hidden="true">…</span>
          ) : (
            <button
              key={item}
              type="button"
              className={`table-pagination-page ${item === currentPage ? 'is-active' : ''}`}
              onClick={() => onPageChange(item)}
              disabled={disabled}
              aria-current={item === currentPage ? 'page' : undefined}
              aria-label={`第 ${item} 页`}
            >
              {item}
            </button>
          ))}
        </div>
        <button
          type="button"
          className="table-pagination-button"
          onClick={() => onPageChange(Math.min(totalPages, currentPage + 1))}
          disabled={disabled || currentPage >= totalPages}
          aria-label="下一页"
        >
          <ChevronRight size={14} />
        </button>
      </div>
    </footer>
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
  totalItems,
  onPageChange,
  paginationDisabled = false,
  emptyState,
  loading = false,
  loadingState,
  className = '',
  tableWrapClassName = '',
  tableClassName = '',
  mobileListClassName = '',
  embedded = false,
}: DataTableProps<T>) {
  const showPagination = typeof currentPage === 'number'
    && typeof totalPages === 'number'
    && totalPages > 0
    && typeof onPageChange === 'function';
  const resolvedTotalItems = typeof totalItems === 'number' ? totalItems : data.length;
  const hasRows = data.length > 0;

  return (
    <div className={`shared-data-table ${embedded ? 'is-embedded' : ''} ${className}`.trim()}>
      {(onSearchChange || filterControls) && (
        <div className="shared-data-table-toolbar">
          {onSearchChange && (
            <label className="shared-data-table-search">
              <Search aria-hidden="true" />
              <input
                type="text"
                placeholder={searchPlaceholder}
                value={searchValue || ''}
                onChange={(event) => onSearchChange(event.target.value)}
              />
            </label>
          )}
          {filterControls && <div className="shared-data-table-filters">{filterControls}</div>}
        </div>
      )}

      <div className="shared-data-table-surface">
        {loading && !hasRows ? (
          loadingState || <div className="shared-data-table-empty">正在加载...</div>
        ) : !hasRows ? (
          emptyState || <div className="shared-data-table-empty">无匹配数据</div>
        ) : (
          <>
            <div className={`shared-data-table-viewport ${renderMobileItem ? 'hidden md:block' : ''} ${tableWrapClassName}`.trim()}>
              <table className={`shared-data-table-grid ${tableClassName}`.trim()}>
                <thead>
                  <tr>
                    {headers.map((header) => (
                      <th key={header.key} className={header.className || ''}>{header.label}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>{data.map((item, index) => renderRow(item, index))}</tbody>
              </table>
            </div>
            {renderMobileItem && (
              <div className={`shared-data-table-mobile block md:hidden ${mobileListClassName}`.trim()}>
                {data.map((item, index) => renderMobileItem(item, index))}
              </div>
            )}
          </>
        )}

        {showPagination && (
          <TablePagination
            currentPage={currentPage as number}
            totalPages={totalPages as number}
            totalItems={resolvedTotalItems}
            onPageChange={onPageChange as (page: number) => void}
            disabled={paginationDisabled}
          />
        )}
      </div>
    </div>
  );
}
