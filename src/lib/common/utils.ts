export function formatDate(dateString: string, includeTime = true): string {
  if (!dateString) return '-';
  const date = new Date(dateString);
  if (isNaN(date.getTime())) return dateString;

  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  const h = String(date.getHours()).padStart(2, '0');
  const min = String(date.getMinutes()).padStart(2, '0');
  const s = String(date.getSeconds()).padStart(2, '0');

  if (includeTime) {
    return `${y}-${m}-${d} ${h}:${min}:${s}`;
  }
  return `${y}-${m}-${d}`;
}

export function formatCNY(amount: number, precision = 2): string {
  if (amount === undefined || amount === null) return '¥0.00';
  return `¥${amount.toFixed(precision)}`;
}
