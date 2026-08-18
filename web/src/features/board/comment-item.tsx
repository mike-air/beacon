/**
 * One comment, with the two things you can do to it.
 *
 * The permission rules are the server's, mirrored here only to decide what to
 * SHOW. Hiding a button is a courtesy, never the enforcement — the API rejects
 * an edit from a non-author and a delete from a non-admin regardless of what
 * this file renders (see tasks.UpdateComment / tasks.DeleteComment). If the
 * two ever disagree, the server wins and the user sees an error, which is the
 * right failure: a UI that permits what the API forbids is annoying, a UI that
 * relies on hidden buttons for security is broken.
 *
 * The rules differ on purpose, and the asymmetry is the interesting part:
 *
 *   edit   — the author alone. An admin rewriting somebody's words under that
 *            person's name has no legitimate use, so the capability does not
 *            exist rather than being guarded.
 *   delete — the author, or an org admin/owner. Moderation has to be possible
 *            by somebody other than whoever posted the thing.
 */
import { Check, Pencil, Trash2 } from "lucide-react";
import { useState } from "react";
import type { Comment, Role } from "@/api/types";
import { isAdmin } from "@/api/types";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";

export function CommentItem({
  comment,
  currentUserID,
  currentRole,
  onEdit,
  onDelete,
  busy,
  when,
}: {
  comment: Comment;
  currentUserID: string | undefined;
  currentRole: Role | undefined;
  onEdit: (body: string) => void;
  onDelete: () => void;
  busy: boolean;
  when: (iso: string) => string;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(comment.body);
  const [confirming, setConfirming] = useState(false);

  const mine = currentUserID !== undefined && comment.author_id === currentUserID;
  const canEdit = mine;
  const canDelete = mine || isAdmin(currentRole);

  function save() {
    const body = draft.trim();
    // An empty edit is a delete the user did not ask for. Treat it as a
    // cancel: the delete button is right there and says what it does.
    if (!body || body === comment.body) {
      setEditing(false);
      setDraft(comment.body);
      return;
    }
    onEdit(body);
    setEditing(false);
  }

  if (editing) {
    return (
      <li className="flex gap-2">
        <Avatar name={comment.author_id.slice(0, 2)} size="sm" />
        <div className="min-w-0 flex-1">
          <textarea
            autoFocus
            rows={2}
            value={draft}
            aria-label="Edit comment"
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              // Enter saves, Shift+Enter is a newline, Escape abandons. The
              // same three keys the composer above uses, so editing does not
              // need its own muscle memory.
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                save();
              } else if (e.key === "Escape") {
                setEditing(false);
                setDraft(comment.body);
              }
            }}
            className="w-full rounded-(--radius-ctl) border border-line bg-page px-2.5 py-1.5 text-ui text-ink focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <div className="mt-1 flex gap-1">
            <Button size="sm" onClick={save} busy={busy}>
              <Check className="size-3" />
              Save
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setEditing(false);
                setDraft(comment.body);
              }}
            >
              Cancel
            </Button>
          </div>
        </div>
      </li>
    );
  }

  return (
    <li className="group flex gap-2">
      <Avatar name={comment.author_id.slice(0, 2)} size="sm" />
      <div className="min-w-0 flex-1">
        <p className="whitespace-pre-wrap break-words text-ui text-ink">{comment.body}</p>
        <p className="mt-0.5 font-mono text-micro text-ink-faint">
          {when(comment.created_at)}
          {/* Shown, not hidden: a reader is entitled to know the text in front
              of them is not what was originally posted. */}
          {comment.edited && <span title={`Edited ${when(comment.updated_at)}`}> · edited</span>}
        </p>

        {confirming ? (
          <div className="mt-1 flex items-center gap-1.5">
            <span className="text-caption text-ink-muted">Delete this comment?</span>
            <Button size="sm" variant="danger" onClick={onDelete} busy={busy}>
              Delete
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setConfirming(false)}>
              Keep
            </Button>
          </div>
        ) : (
          (canEdit || canDelete) && (
            // Revealed on hover and on keyboard focus. focus-within is what
            // keeps these reachable by tab — hover alone would hide them from
            // anyone not using a mouse.
            <div className="mt-0.5 flex gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
              {canEdit && (
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label="Edit comment"
                  onClick={() => setEditing(true)}
                >
                  <Pencil className="size-3" />
                </Button>
              )}
              {canDelete && (
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label="Delete comment"
                  className="text-danger-text"
                  onClick={() => setConfirming(true)}
                >
                  <Trash2 className="size-3" />
                </Button>
              )}
            </div>
          )
        )}
      </div>
    </li>
  );
}
