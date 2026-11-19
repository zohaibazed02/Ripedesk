enum CardAuthOutcome { approved, declined, ambiguous }

class CardAuthRequest {
  const CardAuthRequest({
    required this.paymentId,
    required this.orderId,
    required this.amountMinorUnits,
    required this.currencyCode,
  });

  final String paymentId;
  final String orderId;
  final int amountMinorUnits;
  final String currencyCode;
}

class CardAuthResult {
  const CardAuthResult({
    required this.paymentId,
    required this.outcome,
    this.providerRef,
    this.approvalCode,
    this.message,
  });

  final String paymentId;
  final CardAuthOutcome outcome;
  final String? providerRef;
  final String? approvalCode;
  final String? message;
}

/// Platform-agnostic interface for external EMV/NFC card terminals.
abstract interface class PaymentTerminalAdapter {
  Future<CardAuthResult> authorizeCard(CardAuthRequest request);
  Future<CardAuthResult> voidTransaction(String providerRef);
  Future<CardAuthResult> reconcileAmbiguous(String paymentId);
}
