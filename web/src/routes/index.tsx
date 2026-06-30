import { createRoute } from '@tanstack/react-router';

import { rootRoute } from './root';

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomePage,
});

function HomePage() {
  return (
    <section className="grid gap-6 md:grid-cols-2">
      <div className="rounded-lg border bg-card p-6 text-card-foreground">
        <p className="text-sm font-medium text-muted-foreground">
          Control plane
        </p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight">
          Payment gateway operations
        </h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          Manage apps, channels, routing, payment orders, webhooks, refunds,
          and subscriptions from one interface.
        </p>
      </div>
      <div className="rounded-lg border bg-card p-6 text-card-foreground">
        <p className="text-sm font-medium text-muted-foreground">Stack</p>
        <ul className="mt-3 space-y-2 text-sm text-muted-foreground">
          <li>React 19 + Rsbuild</li>
          <li>TanStack Router + Query</li>
          <li>Tailwind CSS v4</li>
          <li>shadcn UI components</li>
        </ul>
      </div>
    </section>
  );
}
