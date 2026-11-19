import 'package:drift/drift.dart';

class CachedCatalogTable extends Table {
  IntColumn get id => integer().withDefault(const Constant(1))();
  TextColumn get etag => text().withDefault(const Constant(''))();
  TextColumn get catalogJson => text().withDefault(const Constant('{}'))();
  DateTimeColumn get updatedAt => dateTime().withDefault(currentDateAndTime)();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

class LocalOrdersTable extends Table {
  TextColumn get id => text()();
  TextColumn get orderNumber => text()();
  TextColumn get orderMode => text()();
  TextColumn get tableId => text().nullable()();
  IntColumn get orderVersion => integer().withDefault(const Constant(1))();
  TextColumn get status => text().withDefault(const Constant('open'))();
  TextColumn get grandTotal => text().withDefault(const Constant('0.0000'))();
  TextColumn get syncState => text().withDefault(const Constant('LAN_MODE'))();
  DateTimeColumn get updatedAt => dateTime().withDefault(currentDateAndTime)();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

class LocalOrderItemsTable extends Table {
  TextColumn get id => text()();
  TextColumn get orderId => text()();
  IntColumn get roundNumber => integer().withDefault(const Constant(1))();
  TextColumn get menuItemId => text()();
  TextColumn get itemName => text()();
  TextColumn get quantity => text()();
  TextColumn get unitPrice => text()();
  IntColumn get seatNumber => integer().nullable()();
  IntColumn get checkNumber => integer().withDefault(const Constant(1))();
  TextColumn get modifiersJson => text().withDefault(const Constant('[]'))();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

class DeviceOutboxTable extends Table {
  IntColumn get seqId => integer().autoIncrement()();
  TextColumn get commandId => text().unique()();
  TextColumn get idempotencyKey => text().unique()();
  TextColumn get commandType => text()();
  TextColumn get endpoint => text()();
  TextColumn get payloadJson => text()();
  TextColumn get status => text().withDefault(const Constant('pending'))(); // pending | synced | conflict
  IntColumn get retryCount => integer().withDefault(const Constant(0))();
  TextColumn get lastError => text().withDefault(const Constant(''))();
  DateTimeColumn get createdAt => dateTime().withDefault(currentDateAndTime)();
}

class CashShiftsTable extends Table {
  TextColumn get id => text()();
  TextColumn get openedBy => text()();
  TextColumn get closedBy => text().nullable()();
  TextColumn get openingFloat => text().withDefault(const Constant('0.0000'))();
  TextColumn get cashSalesTotal => text().withDefault(const Constant('0.0000'))();
  TextColumn get cashRefundsTotal => text().withDefault(const Constant('0.0000'))();
  TextColumn get expectedCash => text().withDefault(const Constant('0.0000'))();
  TextColumn get countedCash => text().nullable()();
  TextColumn get variance => text().nullable()();
  TextColumn get status => text().withDefault(const Constant('open'))();
  DateTimeColumn get openedAt => dateTime().withDefault(currentDateAndTime)();
  DateTimeColumn get closedAt => dateTime().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// Lightweight Drift-compatible DAO contract implemented against PosDatabase.
abstract interface class PosLocalStore {
  Future<void> saveCatalog({required String etag, required String catalogJson});
  Future<({String etag, String catalogJson})?> loadCatalog();
  Future<int> enqueueOutboxCommand({
    required String commandId,
    required String idempotencyKey,
    required String commandType,
    required String endpoint,
    required String payloadJson,
  });
  Future<void> markOutboxStatus(String commandId, String status, {String lastError = ''});
}
