'use client';

import React from 'react';
import { ChevronLeft, ChevronRight, Search } from 'lucide-react';

interface Header {
  key: string;
  label: string;
  className?: string;
}

interface DataTableProps<T> {
  headers: Header[];
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

  emptyState?: React.ReactNode;
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
  emptyState
}: DataTableProps<T>) {
  const showPagination = !!(currentPage && totalPages && totalPages > 1);

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
      {data.length === 0 ? (
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
                  {headers.map((h) => (
                    <th
                      key={h.key}
                      className={`px-4 py-2.5 text-xs font-semibold text-[#17201B]/60 tracking-wider ${h.className || ''}`}
                    >
                      {h.label}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-[#DCE4DF] text-xs text-[#17201B]">
                {data.map((item, idx) => renderRow(item, idx))}
              </tbody>
            </table>
          </div>

          {/* Mobile Collapsed Cards View */}
          <div className="block md:hidden space-y-3.5">
            {data.map((item, idx) => renderMobileItem(item, idx))}
          </div>
        </>
      )}

      {/* Pagination Footer */}
      {showPagination && currentPage && totalPages && onPageChange && (
        <div className="flex items-center justify-between bg-white px-4 py-3 border border-[#DCE4DF] rounded-md text-xs shadow-sm">
          <div className="text-[#17201B]/60 font-sans">
            第 <span className="font-mono font-semibold text-[#17201B]">{currentPage}</span> 页，共{' '}
            <span className="font-mono font-semibold text-[#17201B]">{totalPages}</span> 页
          </div>
          <div className="flex items-center gap-1.5">
            <button
              onClick={() => onPageChange(currentPage - 1)}
              disabled={currentPage === 1}
              className="p-1 border border-[#DCE4DF] rounded-md bg-white text-[#17201B] hover:bg-zinc-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <button
              onClick={() => onPageChange(currentPage + 1)}
              disabled={currentPage === totalPages}
              className="p-1 border border-[#DCE4DF] rounded-md bg-white text-[#17201B] hover:bg-zinc-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
