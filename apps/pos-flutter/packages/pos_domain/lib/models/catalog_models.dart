import 'package:decimal/decimal.dart';

class Category {
  const Category({
    required this.id,
    required this.name,
    required this.sortOrder,
  });

  final String id;
  final String name;
  final int sortOrder;

  Category copyWith({String? name, int? sortOrder}) => Category(
        id: id,
        name: name ?? this.name,
        sortOrder: sortOrder ?? this.sortOrder,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'sort_order': sortOrder,
      };

  factory Category.fromJson(Map<String, dynamic> json) => Category(
        id: json['id'] as String,
        name: json['name'] as String,
        sortOrder: json['sort_order'] as int? ?? 0,
      );
}

class MenuItem {
  const MenuItem({
    required this.id,
    required this.categoryId,
    required this.sku,
    required this.name,
    required this.basePrice,
    required this.taxRatePercent,
    required this.prepStation,
    required this.isAvailable,
    this.description = '',
    this.modifierGroupIds = const [],
  });

  final String id;
  final String categoryId;
  final String sku;
  final String name;
  final String description;
  final Decimal basePrice;
  final Decimal taxRatePercent;
  final String prepStation;
  final bool isAvailable;
  final List<String> modifierGroupIds;

  MenuItem copyWith({
    Decimal? basePrice,
    bool? isAvailable,
    List<String>? modifierGroupIds,
  }) =>
      MenuItem(
        id: id,
        categoryId: categoryId,
        sku: sku,
        name: name,
        description: description,
        basePrice: basePrice ?? this.basePrice,
        taxRatePercent: taxRatePercent,
        prepStation: prepStation,
        isAvailable: isAvailable ?? this.isAvailable,
        modifierGroupIds: modifierGroupIds ?? this.modifierGroupIds,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'category_id': categoryId,
        'sku': sku,
        'name': name,
        'description': description,
        'base_price': basePrice.toString(),
        'tax_rate_percent': taxRatePercent.toString(),
        'prep_station': prepStation,
        'is_available': isAvailable,
        'modifier_group_ids': modifierGroupIds,
      };

  factory MenuItem.fromJson(Map<String, dynamic> json) => MenuItem(
        id: json['id'] as String,
        categoryId: json['category_id'] as String,
        sku: json['sku'] as String,
        name: json['name'] as String,
        description: json['description'] as String? ?? '',
        basePrice: Decimal.parse(json['base_price'].toString()),
        taxRatePercent: Decimal.parse((json['tax_rate_percent'] ?? '0').toString()),
        prepStation: json['prep_station'] as String? ?? 'hot_kitchen',
        isAvailable: json['is_available'] as bool? ?? true,
        modifierGroupIds: (json['modifier_group_ids'] as List<dynamic>? ?? [])
            .map((e) => e.toString())
            .toList(),
      );
}

class Modifier {
  const Modifier({
    required this.id,
    required this.modifierGroupId,
    required this.name,
    required this.priceDelta,
    required this.isAvailable,
  });

  final String id;
  final String modifierGroupId;
  final String name;
  final Decimal priceDelta;
  final bool isAvailable;

  Map<String, dynamic> toJson() => {
        'id': id,
        'modifier_group_id': modifierGroupId,
        'name': name,
        'price_delta': priceDelta.toString(),
        'is_available': isAvailable,
      };

  factory Modifier.fromJson(Map<String, dynamic> json) => Modifier(
        id: json['id'] as String,
        modifierGroupId: json['modifier_group_id'] as String,
        name: json['name'] as String,
        priceDelta: Decimal.parse((json['price_delta'] ?? '0').toString()),
        isAvailable: json['is_available'] as bool? ?? true,
      );
}

class ModifierGroup {
  const ModifierGroup({
    required this.id,
    required this.name,
    required this.minSelections,
    required this.maxSelections,
    required this.modifiers,
  });

  final String id;
  final String name;
  final int minSelections;
  final int maxSelections;
  final List<Modifier> modifiers;

  bool get required => minSelections > 0;

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'min_selections': minSelections,
        'max_selections': maxSelections,
        'modifiers': modifiers.map((m) => m.toJson()).toList(),
      };

  factory ModifierGroup.fromJson(Map<String, dynamic> json) => ModifierGroup(
        id: json['id'] as String,
        name: json['name'] as String,
        minSelections: json['min_selections'] as int? ?? 0,
        maxSelections: json['max_selections'] as int? ?? 1,
        modifiers: (json['modifiers'] as List<dynamic>? ?? [])
            .map((e) => Modifier.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}

enum TableStatus {
  available('available'),
  occupied('occupied'),
  reserved('reserved'),
  dirty('dirty');

  const TableStatus(this.wireValue);
  final String wireValue;

  static TableStatus fromWire(String? raw) {
    return TableStatus.values.firstWhere(
      (s) => s.wireValue == raw,
      orElse: () => TableStatus.available,
    );
  }
}

class DiningTable {
  const DiningTable({
    required this.id,
    required this.floorId,
    required this.tableCode,
    required this.capacity,
    required this.status,
    this.posX = 0,
    this.posY = 0,
    this.shape = 'rectangle',
    this.activeOrderId,
    this.orderVersion = 0,
  });

  final String id;
  final String floorId;
  final String tableCode;
  final int capacity;
  final int posX;
  final int posY;
  final String shape;
  final TableStatus status;
  final String? activeOrderId;
  final int orderVersion;

  DiningTable copyWith({
    TableStatus? status,
    String? activeOrderId,
    int? orderVersion,
  }) =>
      DiningTable(
        id: id,
        floorId: floorId,
        tableCode: tableCode,
        capacity: capacity,
        posX: posX,
        posY: posY,
        shape: shape,
        status: status ?? this.status,
        activeOrderId: activeOrderId ?? this.activeOrderId,
        orderVersion: orderVersion ?? this.orderVersion,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'floor_id': floorId,
        'table_code': tableCode,
        'capacity': capacity,
        'pos_x': posX,
        'pos_y': posY,
        'shape': shape,
        'status': status.wireValue,
        'active_order_id': activeOrderId,
        'order_version': orderVersion,
      };

  factory DiningTable.fromJson(Map<String, dynamic> json) => DiningTable(
        id: json['id'] as String,
        floorId: json['floor_id'] as String,
        tableCode: json['table_code'] as String,
        capacity: json['capacity'] as int? ?? 2,
        posX: json['pos_x'] as int? ?? 0,
        posY: json['pos_y'] as int? ?? 0,
        shape: json['shape'] as String? ?? 'rectangle',
        status: TableStatus.fromWire(json['status'] as String?),
        activeOrderId: json['active_order_id'] as String?,
        orderVersion: json['order_version'] as int? ?? 0,
      );
}

class DiningFloor {
  const DiningFloor({
    required this.id,
    required this.name,
    required this.sortOrder,
    required this.tables,
  });

  final String id;
  final String name;
  final int sortOrder;
  final List<DiningTable> tables;

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'sort_order': sortOrder,
        'tables': tables.map((t) => t.toJson()).toList(),
      };

  factory DiningFloor.fromJson(Map<String, dynamic> json) => DiningFloor(
        id: json['id'] as String,
        name: json['name'] as String,
        sortOrder: json['sort_order'] as int? ?? 0,
        tables: (json['tables'] as List<dynamic>? ?? [])
            .map((e) => DiningTable.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}
