export function webhookStatusVariant(status: string) {
  switch (status) {
    case "succeeded":
      return "secondary";
    case "failed":
      return "destructive";
    default:
      return "outline";
  }
}
