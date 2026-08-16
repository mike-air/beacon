import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Button } from "./button";

describe("Button", () => {
  it("is a real button that reports its accessible name", () => {
    render(<Button>New task</Button>);
    expect(screen.getByRole("button", { name: "New task" })).toBeInTheDocument();
  });

  it("busy implies disabled, so a double submit is impossible", async () => {
    const onClick = vi.fn();
    render(<Button busy onClick={onClick}>Saving</Button>);
    const btn = screen.getByRole("button", { name: /Saving/ });
    expect(btn).toBeDisabled();
    await userEvent.click(btn, { pointerEventsCheck: 0 });
    expect(onClick).not.toHaveBeenCalled();
  });

  /**
   * asChild exists so a link can look like a button without a <button>
   * wrapping an <a> — invalid HTML that confuses every screen reader.
   */
  it("asChild renders the child element, not a nested button", () => {
    const { container } = render(
      <Button asChild>
        <a href="/board">Back to your board</a>
      </Button>,
    );
    expect(screen.getByRole("link", { name: "Back to your board" })).toBeInTheDocument();
    expect(container.querySelector("button")).toBeNull();
  });

  it("defaults to type=button so it cannot submit a form by accident", () => {
    render(<Button>Filter</Button>);
    expect(screen.getByRole("button", { name: "Filter" })).toHaveAttribute("type", "button");
  });
});
