import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Highlight } from "./highlight";

/**
 * Meilisearch wraps matches in <mark>. The search index holds text other
 * people typed, so this component is a security boundary: it must render the
 * markers and nothing else as markup.
 */
describe("Highlight", () => {
  it("renders the markers as real <mark> elements", () => {
    const { container } = render(<Highlight text="Draft the <mark>launch</mark> plan" />);
    const marks = container.querySelectorAll("mark");
    expect(marks).toHaveLength(1);
    expect(marks[0]!.textContent).toBe("launch");
    expect(container.textContent).toBe("Draft the launch plan");
  });

  it("renders injected HTML as text, creating no elements", () => {
    const { container } = render(
      <Highlight text='<mark>launch</mark> <img src=x onerror=alert(1)> audit' />,
    );
    expect(container.querySelectorAll("img")).toHaveLength(0);
    expect(container.querySelectorAll("mark")).toHaveLength(1);
    expect(container.textContent).toContain("<img src=x onerror=alert(1)>");
  });

  it("neutralises a script tag", () => {
    const { container } = render(<Highlight text="<script>alert(1)</script>" />);
    expect(container.querySelector("script")).toBeNull();
    expect(container.textContent).toBe("<script>alert(1)</script>");
  });

  it("handles several matches and text with no matches", () => {
    const { container } = render(<Highlight text="<mark>a</mark> b <mark>c</mark>" />);
    expect(container.querySelectorAll("mark")).toHaveLength(2);
    render(<Highlight text="plain, from the postgres fallback" />);
    expect(screen.getByText("plain, from the postgres fallback")).toBeInTheDocument();
  });
});
