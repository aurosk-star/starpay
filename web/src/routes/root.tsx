import {
  createRootRoute,
  Outlet,
} from '@tanstack/react-router';

export const rootRoute = createRootRoute({
  component: () => (
    <div className="min-h-[100dvh] bg-background text-foreground">
      <header className="border-b">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-6 py-4">
          <div className="font-semibold">Payment Gateway</div>
          <nav className="flex items-center gap-4 text-sm text-muted-foreground">
            <a href="/" className="hover:text-foreground">
              Dashboard
            </a>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-7xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  ),
});
