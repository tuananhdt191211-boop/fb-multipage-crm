export function formatVnd(n: number): string {
  return new Intl.NumberFormat('vi-VN').format(Math.max(0, Math.round(n || 0))) + 'đ'
}