import type { Metadata } from "next";
import "@daypicker/react/style.css";
import "./globals.css";
import { Toaster } from "sonner";
import { SupportWidget } from "@/components/common/SupportWidget";

export const metadata: Metadata = {
  title: "AIπ 图片中转站",
  description: "统一管理图片请求、转发渠道、调用记录与账户余额。",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN" className="h-full">
      <body className="min-h-full bg-[#F6F8F6] text-[#17201B]">
        {children}
        <SupportWidget />
        <Toaster position="top-right" richColors />
      </body>
    </html>
  );
}
