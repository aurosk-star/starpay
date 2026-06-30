import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertTriangle,
  Plus,
  RefreshCw,
  ShieldCheck,
  Pencil,
  Trash2,
  Users,
} from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  createDataTable,
  DataTableRowActions,
  type DataTableColumn,
} from "@/components/data-table";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { APIError } from "@/lib/api";
import { useAuthStore } from "@/features/auth/store";
import {
  createUser,
  deleteUser,
  listRoles,
  listUsers,
  updateUser,
} from "@/features/auth/api";
import type { AdminUser, Role } from "@/features/auth/types";

const emptyForm = {
  username: "",
  email: "",
  password: "",
  displayName: "",
  status: "enabled",
  roleIds: [] as number[],
};

type FormState = typeof emptyForm;

const UsersDataTable = createDataTable<AdminUser>();

export function UsersPage() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<AdminUser | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<AdminUser | null>(null);
  const userColumns = useMemo<DataTableColumn<AdminUser>[]>(
    () => [
      {
        accessorKey: "username",
        header: t("users.table.user"),
        cell: ({ row }) => {
          const user = row.original;
          return (
            <div className="flex items-center gap-3">
              <Avatar className="size-8">
                <AvatarFallback>
                  {user.username.slice(0, 2).toUpperCase()}
                </AvatarFallback>
              </Avatar>
              <div>
                <p className="font-medium">
                  {user.display_name || user.username}
                </p>
                <p className="text-xs text-muted-foreground">{user.username}</p>
              </div>
            </div>
          );
        },
      },
      {
        accessorKey: "email",
        header: t("users.table.email"),
      },
      {
        accessorKey: "roles",
        header: t("users.table.roles"),
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            {row.original.roles.map((role) => (
              <Badge key={role} variant="outline">
                {role}
              </Badge>
            ))}
          </div>
        ),
      },
      {
        accessorKey: "status",
        header: t("users.table.status"),
        cell: ({ row }) => (
          <Badge
            variant={
              row.original.status === "enabled" ? "secondary" : "outline"
            }
          >
            {row.original.status === "enabled"
              ? t("users.enabled")
              : row.original.status}
          </Badge>
        ),
      },
      {
        id: "actions",
        header: () => (
          <div className="text-right">{t("common.moreActions")}</div>
        ),
        cell: ({ row }) => (
          <DataTableRowActions
            actions={[
              {
                label: t("users.edit"),
                icon: Pencil,
                onClick: () => openEdit(row.original),
              },
              {
                label: t("users.disable"),
                icon: Trash2,
                onClick: () => setDeleteTarget(row.original),
              },
            ]}
          />
        ),
      },
    ],
    [t, roles],
  );

  async function load() {
    if (!accessToken) return;
    setLoading(true);
    setError(null);
    try {
      const [userResult, roleResult] = await Promise.all([
        listUsers(accessToken),
        listRoles(accessToken),
      ]);
      setUsers(userResult.items);
      setRoles(roleResult.items);
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.message);
      } else {
        setError(t("users.loadFailed"));
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [accessToken]);

  function openCreate() {
    setEditingUser(null);
    setForm(emptyForm);
    setDialogOpen(true);
  }

  function openEdit(user: AdminUser) {
    setEditingUser(user);
    setForm({
      username: user.username,
      email: user.email,
      password: "",
      displayName: user.display_name || "",
      status: user.status,
      roleIds: roles
        .filter((role) => user.roles.includes(role.code))
        .map((role) => role.id),
    });
    setDialogOpen(true);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) return;
    setSaving(true);
    try {
      const payload = {
        username: form.username,
        email: form.email,
        password: form.password || undefined,
        display_name: form.displayName || undefined,
        status: form.status,
        role_ids: form.roleIds,
      };
      const result = editingUser
        ? await updateUser(accessToken, editingUser.id, payload)
        : await createUser(accessToken, payload);
      setUsers((current) =>
        editingUser
          ? current.map((item) =>
              item.id === result.user.id ? result.user : item,
            )
          : [result.user, ...current],
      );
      setDialogOpen(false);
      setEditingUser(null);
      setForm(emptyForm);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("users.saveFailed"));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(user: AdminUser) {
    if (!accessToken) return;
    try {
      await deleteUser(accessToken, user.id);
      setUsers((current) =>
        current.map((item) =>
          item.id === user.id ? { ...item, status: "disabled" } : item,
        ),
      );
      setDeleteTarget(null);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t("users.deleteFailed"));
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <section className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <p className="text-sm text-muted-foreground">访问控制</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">
            {t("users.title")}
          </h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
            {t("users.description")}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={() => void load()}
            disabled={loading}
          >
            <RefreshCw data-icon="inline-start" />
            {t("common.refresh")}
          </Button>
          <Button onClick={openCreate}>
            <Plus data-icon="inline-start" />
            {t("users.create")}
          </Button>
        </div>
      </section>

      {error ? (
        <Alert variant="destructive">
          <AlertTriangle />
          <AlertTitle>{t("users.loadFailed")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <section className="grid gap-4 lg:grid-cols-[1fr_360px]">
        <CardPanel
          title={t("users.adminUsersTitle")}
          description={t("users.adminUsers")}
          count={t("users.userCount", { count: users.length })}
        >
          <UsersDataTable
            columns={userColumns}
            data={users}
            loading={loading}
            loadingText={t("users.loading")}
            emptyText={t("users.empty")}
          />
        </CardPanel>

        <CardPanel
          title={t("users.defaultRolesTitle")}
          description={t("users.defaultRoles")}
        >
          <div className="flex flex-col gap-3">
            {roles.map((role) => (
              <Card key={role.id}>
                <CardContent className="py-4">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2">
                      {role.code === "super_admin" ? (
                        <ShieldCheck className="size-4 text-muted-foreground" />
                      ) : (
                        <Users className="size-4 text-muted-foreground" />
                      )}
                      <p className="text-sm font-medium">{role.name}</p>
                    </div>
                    <Badge variant="outline">{role.code}</Badge>
                  </div>
                  <p className="mt-2 text-xs leading-5 text-muted-foreground">
                    {role.description || t("users.noDescription")}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        </CardPanel>
      </section>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingUser ? t("users.editTitle") : t("users.createTitle")}
            </DialogTitle>
            <DialogDescription>{t("users.formDescription")}</DialogDescription>
          </DialogHeader>
          <form className="flex flex-col gap-5" onSubmit={handleSubmit}>
            <FieldGroup>
              <div className="grid gap-4 md:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="username">
                    {t("auth.username")}
                  </FieldLabel>
                  <Input
                    id="username"
                    value={form.username}
                    onChange={(e) =>
                      setForm((current) => ({
                        ...current,
                        username: e.target.value,
                      }))
                    }
                    required
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="email">{t("auth.email")}</FieldLabel>
                  <Input
                    id="email"
                    value={form.email}
                    type="email"
                    onChange={(e) =>
                      setForm((current) => ({
                        ...current,
                        email: e.target.value,
                      }))
                    }
                    required
                  />
                </Field>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="password">
                    {t("auth.password")}
                  </FieldLabel>
                  <Input
                    id="password"
                    value={form.password}
                    type="password"
                    onChange={(e) =>
                      setForm((current) => ({
                        ...current,
                        password: e.target.value,
                      }))
                    }
                    placeholder={
                      editingUser ? t("users.passwordPlaceholder") : ""
                    }
                    required={!editingUser}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="displayName">
                    {t("auth.displayName")}
                  </FieldLabel>
                  <Input
                    id="displayName"
                    value={form.displayName}
                    onChange={(e) =>
                      setForm((current) => ({
                        ...current,
                        displayName: e.target.value,
                      }))
                    }
                  />
                </Field>
              </div>
              <Field>
                <FieldLabel>{t("users.status")}</FieldLabel>
                <Select
                  value={form.status}
                  onValueChange={(value) =>
                    setForm((current) => ({ ...current, status: value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="enabled">
                        {t("users.enabled")}
                      </SelectItem>
                      <SelectItem value="disabled">
                        {t("users.disabled")}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel>{t("users.roles")}</FieldLabel>
                <div className="flex flex-wrap gap-3">
                  {roles.map((role) => {
                    const checked = form.roleIds.includes(role.id);
                    return (
                      <label
                        key={role.id}
                        className={`flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors ${checked ? "border-primary bg-primary/5" : "bg-background"}`}
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={() =>
                            setForm((current) => ({
                              ...current,
                              roleIds: checked
                                ? current.roleIds.filter((id) => id !== role.id)
                                : [...current.roleIds, role.id],
                            }))
                          }
                        />
                        <span className="text-sm font-medium">{role.name}</span>
                      </label>
                    );
                  })}
                </div>
              </Field>
            </FieldGroup>
            <div className="flex items-center justify-between gap-3">
              <FieldError>{error}</FieldError>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setDialogOpen(false)}
                >
                  {t("users.cancel")}
                </Button>
                <Button type="submit" disabled={saving}>
                  {saving ? t("auth.processing") : t("users.save")}
                </Button>
              </div>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("users.disable")}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget
                ? t("users.deleteConfirm", { username: deleteTarget.username })
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("users.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deleteTarget) {
                  void handleDelete(deleteTarget);
                }
              }}
            >
              {t("users.disable")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function CardPanel({
  title,
  description,
  count,
  children,
}: {
  title: string;
  description: string;
  count?: string;
  children: React.ReactNode;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-3">
        <div>
          <CardDescription>{description}</CardDescription>
          <CardTitle className="text-lg">{title}</CardTitle>
        </div>
        {count ? <Badge variant="secondary">{count}</Badge> : null}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}
