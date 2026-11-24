import { cn } from "@/lib/utils";

type MainProps = React.HTMLAttributes<HTMLElement> & {
  fixed?: boolean;
  fluid?: boolean;
  ref?: React.Ref<HTMLElement>;
};

export function Main({
  fixed,
  className,
  fluid,
  id = "content",
  ...props
}: MainProps) {
  return (
    <main
      id={id}
      data-layout={fixed ? "fixed" : "auto"}
      className={cn(
        "px-4 py-6",

        // If layout is fixed, make the main container flex and grow.
        // 使用 overflow-y-auto 让超出视口的内容可以纵向滚动（此前 overflow-hidden 会直接裁掉表单底部）。
        fixed && "flex min-h-0 grow flex-col overflow-y-auto",

        // If layout is not fluid, set the max-width
        !fluid &&
          "@7xl/content:mx-auto @7xl/content:w-full @7xl/content:max-w-7xl",
        className,
      )}
      {...props}
    />
  );
}
