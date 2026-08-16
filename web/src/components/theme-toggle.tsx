import { Monitor, Moon, Sun } from "lucide-react";
import { useState } from "react";
import { MenuContent, MenuItem, MenuRoot, MenuTrigger } from "@/components/ui/menu";
import { Button } from "@/components/ui/button";
import { getPreference, setPreference, type ThemePreference } from "@/lib/theme";

const options: { value: ThemePreference; label: string; Icon: typeof Sun }[] = [
  { value: "light", label: "Light", Icon: Sun },
  { value: "dark", label: "Dark", Icon: Moon },
  { value: "system", label: "System", Icon: Monitor },
];

export function ThemeToggle() {
  const [pref, setPref] = useState<ThemePreference>(getPreference);
  const Current = options.find((o) => o.value === pref)?.Icon ?? Monitor;

  return (
    <MenuRoot>
      <MenuTrigger asChild>
        <Button variant="ghost" size="sm" aria-label="Theme">
          <Current className="size-4" />
        </Button>
      </MenuTrigger>
      <MenuContent align="end">
        {options.map(({ value, label, Icon }) => (
          <MenuItem
            key={value}
            onSelect={() => {
              setPreference(value);
              setPref(value);
            }}
          >
            <Icon className="size-3.5 text-ink-faint" />
            {label}
            {pref === value && <span className="ml-auto text-accent-text">•</span>}
          </MenuItem>
        ))}
      </MenuContent>
    </MenuRoot>
  );
}
