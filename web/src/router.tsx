import { createRouter } from '@tanstack/react-router';

import { indexRoute } from './routes/index';
import { rootRoute } from './routes/root';

const routeTree = rootRoute.addChildren([indexRoute]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
