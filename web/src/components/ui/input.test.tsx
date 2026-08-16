import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Field, Input } from "./input";

/**
 * Field generates the id and requires the label as a prop, so a caller
 * cannot forget to associate them.
 */
describe("Field", () => {
  it("binds its label to its control", () => {
    render(<Field label="Email">{(p) => <Input {...p} />}</Field>);
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
  });

  it("announces an error and marks the input invalid", () => {
    render(<Field label="Slug" error="Already taken.">{(p) => <Input {...p} />}</Field>);
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Already taken.");
    const input = screen.getByLabelText("Slug");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAttribute("aria-describedby", alert.id);
  });

  it("describes the input with its hint when there is no error", () => {
    render(<Field label="Email" hint="Invites go out immediately.">{(p) => <Input {...p} />}</Field>);
    expect(screen.queryByRole("alert")).toBeNull();
    const input = screen.getByLabelText("Email");
    expect(input).toHaveAttribute("aria-describedby");
    expect(input).not.toHaveAttribute("aria-invalid");
  });
});
