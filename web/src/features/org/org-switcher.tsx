import { Check, ChevronsUpDown, Plus } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { MenuContent, MenuItem, MenuLabel, MenuRoot, MenuSeparator, MenuTrigger } from "@/components/ui/menu";
import type { OrgWithRole } from "@/api/types";

export function OrgSwitcher({
  orgs,
  activeID,
  onSelect,
  onCreate,
}: {
  orgs: OrgWithRole[];
  activeID: string | undefined;
  onSelect: (orgID: string) => void;
  onCreate: () => void;
}) {
  const active = orgs.find((o) => o.id === activeID);

  return (
    <MenuRoot>
      <MenuTrigger asChild>
        <Button variant="ghost" size="sm" className="gap-1.5 font-medium text-ink">
          <span className="max-w-40 truncate">{active?.name ?? "Choose an organisation"}</span>
          <ChevronsUpDown className="size-3.5 text-ink-faint" />
        </Button>
      </MenuTrigger>
      <MenuContent align="start" className="min-w-56">
        <MenuLabel>Organisations</MenuLabel>
        {orgs.map((o) => (
          <MenuItem key={o.id} onSelect={() => onSelect(o.id)}>
            <span className="truncate">{o.name}</span>
            <Badge tone={o.role === "member" ? "neutral" : "accent"} className="ml-auto">
              {o.role}
            </Badge>
            {o.id === activeID && <Check className="size-3.5 text-accent-text" />}
          </MenuItem>
        ))}
        <MenuSeparator />
        <MenuItem onSelect={onCreate}>
          <Plus className="size-3.5 text-ink-faint" />
          New organisation
        </MenuItem>
      </MenuContent>
    </MenuRoot>
  );
}
