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

/** The 12-character minimum is the server's rule (signupRequest, min=12). It is
 *  repeated here so the user learns it before submitting, not after. */
const schema = z.object({
  email: z.string().min(1, "Email is required").email("That is not an email address"),
  name: z.string().max(200, "That name is too long"),
  password: z
    .string()
    .min(12, "Passwords must be at least 12 characters")
    .max(200, "That password is too long"),
});
type Values = z.infer<typeof schema>;

export function SignUp() {
  const { signUp } = useAuth();
  const navigate = useNavigate();
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    mode: "onBlur",
    defaultValues: { email: "", name: "", password: "" },
  });
  const submit = useSubmit<Values>(form.setError, ["email", "name", "password"]);

  return (
    <AuthLayout
      title="Create your account"
      subtitle="Two minutes and you will have a board with something on it."
      footer={
        <>
          Already have one?{" "}
          <Link to="/sign-in" className="font-medium text-accent-text hover:underline">
            Sign in
          </Link>
        </>
      }
    >
      <form
        noValidate
        onSubmit={form.handleSubmit(async (v) => {
          const ok = await submit.run(async () => {
            await signUp(v.email, v.name, v.password);
          });
          if (ok) await navigate({ to: "/welcome" });
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
          <Field label="Name" error={form.formState.errors.name?.message}>
            {(p) => (
              <Input {...p} autoComplete="name" placeholder="Your name" {...form.register("name")} />
            )}
          </Field>
          <Field
            label="Password"
            error={form.formState.errors.password?.message}
            hint="At least 12 characters."
          >
            {(p) => (
              <Input
                {...p}
                type="password"
                autoComplete="new-password"
                {...form.register("password")}
              />
            )}
          </Field>
          <Button type="submit" className="w-full" busy={form.formState.isSubmitting}>
            Create account
          </Button>
        </div>
      </form>
    </AuthLayout>
  );
}
