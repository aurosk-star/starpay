import type { PaymentOrder } from "./types";

export function canCloseOrder(order: PaymentOrder) {
  return order.status === "pending" || order.status === "failed";
}

export function orderStatusVariant(status: PaymentOrder["status"]) {
  switch (status) {
    case "paid":
      return "secondary";
    case "closed":
      return "outline";
    case "failed":
      return "destructive";
    default:
      return "outline";
  }
}
