import { zodResolver } from "@hookform/resolvers/zod";
import { Link, useNavigate } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/input";
import { AuthLayout } from "./auth-layout";
import { FormError } from "./form-error";
import { useAuth } from "./auth-context";
import { useSubmit } from "./use-submit";

const schema = z.object({
  email: z.string().min(1, "Email is required").email("That is not an email address"),
  password: z.string().min(1, "Password is required"),
});
type Values = z.infer<typeof schema>;

export function SignIn() {
  const { signIn } = useAuth();
  const navigate = useNavigate();
  const form = useForm<Values>({ resolver: zodResolver(schema), mode: "onBlur" });
  const submit = useSubmit<Values>(form.setError, ["email", "password"]);

  return (
    <AuthLayout
      title="Sign in"
      subtitle="Beacon keeps your team's work in one place."
      footer={
        <>
          No account?{" "}
          <Link to="/sign-up" className="font-medium text-accent-text hover:underline">
            Create one
          </Link>
        </>
      }
    >
      <form
        noValidate
        onSubmit={form.handleSubmit(async (v) => {
          const ok = await submit.run(async () => {
            await signIn(v.email, v.password);
          });
          if (ok) await navigate({ to: "/" });
        })}
      >
        <FormError error={submit.error} />
        <div className="space-y-4">
          <Field label="Email" error={form.formState.errors.email?.message}>
            {(p) => (
              <Input
                {...p}
                type="email"
                autoComplete="email"
                autoFocus
                placeholder="you@company.com"
                {...form.register("email")}
              />
            )}
          </Field>
          <Field label="Password" error={form.formState.errors.password?.message}>
            {(p) => (
              <Input
                {...p}
                type="password"
                autoComplete="current-password"
                {...form.register("password")}
              />
            )}
          </Field>
          <Button type="submit" className="w-full" busy={form.formState.isSubmitting}>
            Sign in
          </Button>
        </div>
      </form>
    </AuthLayout>
  );
}
