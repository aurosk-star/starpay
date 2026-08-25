import { Moon, Sun } from "lucide-react";

import { useTheme } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";

export function ThemeSwitch({ label = "Toggle theme" }: { label?: string }) {
  const { theme, setTheme } = useTheme();
  const isDark = theme === "dark";
  return (
    <Button
      variant="outline"
      size="icon-sm"
      aria-label={label}
      onClick={() => setTheme(isDark ? "light" : "dark")}
    >
      {isDark ? <Sun /> : <Moon />}
    </Button>
  );
}
