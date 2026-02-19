"use client";

import dynamic from "next/dynamic";

const PageContent = dynamic(() => import("./PageContent"), { ssr: false });

export default function ClientApp() {
  return <PageContent />;
}
