import type { ComponentProps, PropsWithChildren } from "react";

import { cn } from "@/lib/utils";

export type MainProps = PropsWithChildren<{
  fixed?: boolean;
  fluid?: boolean;
  className?: string;
}> &
  Omit<ComponentProps<"main">, "id" | "className" | "children">;

export function Main({
  fixed = false,
  fluid = false,
  className,
  children,
  ...props
}: MainProps) {
  return (
    <main
      id="main-content"
      tabIndex={-1}
      className={cn(
        "min-w-0 px-3 py-4 sm:px-4 sm:py-5 md:px-6",
        fluid ? "w-full max-w-none" : "mx-auto w-full max-w-7xl",
        fixed && "h-full overflow-hidden",
        className,
      )}
      {...props}
    >
      {children}
    </main>
  );
}
