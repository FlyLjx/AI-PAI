import type { Metadata } from 'next';
import { Toaster } from 'sonner';
import './globals.css';

export const metadata: Metadata = {
  title: 'AI-PAI 管理控制台',
  description: 'AI-PAI API 中转站独立管理后台。',
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="zh-CN" className="h-full">
      <body className="min-h-full bg-[#F6F8F6] text-[#17201B]">
        {children}
        <Toaster position="top-right" richColors />
      </body>
    </html>
  );
}
