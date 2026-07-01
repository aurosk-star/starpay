import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableRow } from "@/components/ui/table";

export function DetailCard({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description ? (
          <CardDescription className="break-all">{description}</CardDescription>
        ) : null}
      </CardHeader>
      <CardContent className="min-w-0">{children}</CardContent>
    </Card>
  );
}

export function DetailTable({ rows }: { rows: Array<[string, string]> }) {
  return (
    <Table className="min-w-[560px]">
      <TableBody>
        {rows.map(([label, value]) => (
          <TableRow key={label}>
            <TableCell className="w-32 text-muted-foreground sm:w-44">
              {label}
            </TableCell>
            <TableCell className="whitespace-normal break-all font-mono text-xs">
              {value || "-"}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

export function ReadOnlyBlock({
  label,
  value,
  monospace,
}: {
  label: string;
  value: string;
  monospace?: boolean;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <div className="text-sm font-medium text-muted-foreground">{label}</div>
      <div
        className={
          monospace
            ? "min-w-0 rounded-md border bg-muted px-3 py-2 font-mono text-xs break-all"
            : "min-w-0 rounded-md border bg-muted px-3 py-2 text-sm break-words"
        }
      >
        {value}
      </div>
    </div>
  );
}

export function DetailSkeleton() {
  return (
    <div className="grid min-w-0 gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-36" />
          <Skeleton className="h-4 w-64" />
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-9 w-full" />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-28" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-48 w-full" />
        </CardContent>
      </Card>
    </div>
  );
}
