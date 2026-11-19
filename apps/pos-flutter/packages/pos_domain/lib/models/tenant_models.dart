import 'order_models.dart';

class BranchConfig {
  const BranchConfig({
    required this.tenantId,
    required this.branchId,
    required this.branchName,
    required this.branchCode,
    required this.branchType,
    required this.defaultOrderMode,
    required this.currencyCode,
    this.taxInclusive = false,
    this.isolatedModeEnabled = false,
  });

  final String tenantId;
  final String branchId;
  final String branchName;
  final String branchCode;
  final String branchType; // coffee_shop | dine_in | qsr | central_warehouse
  final OrderMode defaultOrderMode;
  final String currencyCode;
  final bool taxInclusive;
  final bool isolatedModeEnabled;

  BranchConfig copyWith({
    String? branchName,
    OrderMode? defaultOrderMode,
    bool? taxInclusive,
    bool? isolatedModeEnabled,
  }) {
    return BranchConfig(
      tenantId: tenantId,
      branchId: branchId,
      branchName: branchName ?? this.branchName,
      branchCode: branchCode,
      branchType: branchType,
      defaultOrderMode: defaultOrderMode ?? this.defaultOrderMode,
      currencyCode: currencyCode,
      taxInclusive: taxInclusive ?? this.taxInclusive,
      isolatedModeEnabled: isolatedModeEnabled ?? this.isolatedModeEnabled,
    );
  }

  Map<String, dynamic> toJson() => {
        'tenant_id': tenantId,
        'branch_id': branchId,
        'branch_name': branchName,
        'branch_code': branchCode,
        'branch_type': branchType,
        'default_order_mode': defaultOrderMode.wireValue,
        'currency_code': currencyCode,
        'tax_inclusive': taxInclusive,
        'isolated_mode_enabled': isolatedModeEnabled,
      };

  factory BranchConfig.fromJson(Map<String, dynamic> json) {
    return BranchConfig(
      tenantId: json['tenant_id'] as String,
      branchId: json['branch_id'] as String,
      branchName: json['branch_name'] as String,
      branchCode: json['branch_code'] as String,
      branchType: json['branch_type'] as String? ?? 'coffee_shop',
      defaultOrderMode: OrderMode.fromWire(json['default_order_mode'] as String?),
      currencyCode: json['currency_code'] as String? ?? 'PKR',
      taxInclusive: json['tax_inclusive'] as bool? ?? false,
      isolatedModeEnabled: json['isolated_mode_enabled'] as bool? ?? false,
    );
  }
}

class StaffSession {
  const StaffSession({
    required this.userId,
    required this.name,
    required this.role,
    required this.canOverride,
    required this.pinVerifiedAt,
    required this.expiresAt,
  });

  final String userId;
  final String name;
  final String role;
  final bool canOverride;
  final DateTime pinVerifiedAt;
  final DateTime expiresAt;

  bool isValidAt(DateTime nowUtc) => nowUtc.isBefore(expiresAt);

  StaffSession copyWith({DateTime? pinVerifiedAt, DateTime? expiresAt}) {
    return StaffSession(
      userId: userId,
      name: name,
      role: role,
      canOverride: canOverride,
      pinVerifiedAt: pinVerifiedAt ?? this.pinVerifiedAt,
      expiresAt: expiresAt ?? this.expiresAt,
    );
  }

  Map<String, dynamic> toJson() => {
        'user_id': userId,
        'name': name,
        'role': role,
        'can_override': canOverride,
        'pin_verified_at': pinVerifiedAt.toUtc().toIso8601String(),
        'expires_at': expiresAt.toUtc().toIso8601String(),
      };

  factory StaffSession.fromJson(Map<String, dynamic> json) {
    return StaffSession(
      userId: json['user_id'] as String,
      name: json['name'] as String,
      role: json['role'] as String,
      canOverride: json['can_override'] as bool? ?? false,
      pinVerifiedAt: DateTime.parse(json['pin_verified_at'] as String).toUtc(),
      expiresAt: DateTime.parse(json['expires_at'] as String).toUtc(),
    );
  }
}
