import { Link } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { BrokenBeacon } from "@/components/ui/illustration";

export function NotFound() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-sunken">
      <EmptyState
        illustration={<BrokenBeacon />}
        title="No light here"
        description="That page does not exist, or it belongs to an organisation you are not in."
        action={
          <Button asChild>
            <Link to="/">Back to your board</Link>
          </Button>
        }
      />
    </div>
  );
}
