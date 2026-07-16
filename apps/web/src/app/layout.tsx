import type { Metadata } from "next";
import "@daypicker/react/style.css";
import "./globals.css";
import { Toaster } from "sonner";

export const metadata: Metadata = {
  title: "AI-PAI - 图像 API 中转站",
  description: "统一管理图像 API、上游模型、调用用量、订阅额度与账户余额。",
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
        <Toaster position="top-right" richColors />
      </body>
    </html>
  );
}
