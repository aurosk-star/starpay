import { RouterProvider } from '@tanstack/react-router';

import { Toaster } from '@/components/ui/sonner';

import { router } from './router';

function App() {
  return (
    <>
      <RouterProvider router={router} />
      <Toaster richColors />
    </>
  );
}

export default App;
