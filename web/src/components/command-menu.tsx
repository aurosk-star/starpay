import { useEffect, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Command as CommandIcon, Moon, Search, Sun } from "lucide-react";

import { useTheme } from "@/components/theme-provider";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from "@/components/ui/command";
import { Button } from "@/components/ui/button";
import { getNavigationItems } from "@/config/navigation";

export function CommandMenu() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { setTheme } = useTheme();
  const [open, setOpen] = useState(false);
  const navigation = getNavigationItems(t);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setOpen((current) => !current);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  function goTo(url: string) {
    setOpen(false);
    void navigate({ to: url as never });
  }

  return (
    <>
      <Button
        variant="outline"
        size="sm"
        className="hidden min-w-36 justify-between gap-2 text-muted-foreground sm:inline-flex"
        onClick={() => setOpen(true)}
        aria-label={t("command.open")}
      >
        <span className="flex items-center gap-2">
          <Search className="size-4" />
          <span>{t("command.search")}</span>
        </span>
        <CommandShortcut>⌘K</CommandShortcut>
      </Button>
      <Button
        variant="outline"
        size="icon-sm"
        className="sm:hidden"
        onClick={() => setOpen(true)}
        aria-label={t("command.open")}
      >
        <CommandIcon />
      </Button>
      <CommandDialog
        open={open}
        onOpenChange={setOpen}
        title={t("command.title")}
        description={t("command.description")}
      >
        <CommandInput placeholder={t("command.placeholder")} />
        <CommandList>
          <CommandEmpty>{t("command.empty")}</CommandEmpty>
          {navigation.groups.map((group) => (
            <CommandGroup key={group.titleKey} heading={group.titleKey}>
              {group.items
                .filter((item) => !item.disabled)
                .map((item) => (
                  <CommandItem key={item.url} onSelect={() => goTo(item.url)}>
                    <item.icon />
                    <span>{item.titleKey}</span>
                  </CommandItem>
                ))}
            </CommandGroup>
          ))}
          <CommandSeparator />
          <CommandGroup heading={t("command.theme")}>
            <CommandItem
              onSelect={() => {
                setTheme("light");
                setOpen(false);
              }}
            >
              <Sun />
              <span>{t("command.light")}</span>
            </CommandItem>
            <CommandItem
              onSelect={() => {
                setTheme("dark");
                setOpen(false);
              }}
            >
              <Moon />
              <span>{t("command.dark")}</span>
            </CommandItem>
            <CommandItem
              onSelect={() => {
                setTheme("system");
                setOpen(false);
              }}
            >
              <CommandIcon />
              <span>{t("command.system")}</span>
            </CommandItem>
          </CommandGroup>
        </CommandList>
      </CommandDialog>
    </>
  );
}
