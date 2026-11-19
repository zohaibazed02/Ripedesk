import 'package:decimal/decimal.dart';
import 'package:pos_domain/pos_domain.dart';

class PilotCatalog {
  static const BranchConfig defaultBranch = BranchConfig(
    tenantId: 'tn-artisan-roast',
    branchId: 'br-f7-01',
    branchName: 'F-7 Coffee Bar',
    branchCode: 'ARB-F7',
    branchType: 'coffee_shop',
    defaultOrderMode: OrderMode.quickSale,
    currencyCode: 'PKR',
    taxInclusive: false,
    isolatedModeEnabled: true,
  );

  static const BranchConfig dineInBranch = BranchConfig(
    tenantId: 'tn-artisan-roast',
    branchId: 'br-g9-02',
    branchName: 'G-9 Flagship Dine-In',
    branchCode: 'ARB-G9',
    branchType: 'dine_in',
    defaultOrderMode: OrderMode.dineIn,
    currencyCode: 'PKR',
    taxInclusive: false,
    isolatedModeEnabled: true,
  );

  static const List<BranchConfig> pilotBranches = [defaultBranch, dineInBranch];

  static const List<DiningFloor> dineInFloors = [
    DiningFloor(
      id: 'fl-main',
      name: 'Main Dining Hall',
      sortOrder: 1,
      tables: [
        DiningTable(id: 'tbl-t1', floorId: 'fl-main', tableCode: 'T1', capacity: 2, posX: 24, posY: 24, shape: 'circle', status: TableStatus.available),
        DiningTable(id: 'tbl-t2', floorId: 'fl-main', tableCode: 'T2', capacity: 4, posX: 164, posY: 24, shape: 'rectangle', status: TableStatus.available),
        DiningTable(id: 'tbl-t3', floorId: 'fl-main', tableCode: 'T3', capacity: 4, posX: 324, posY: 24, shape: 'rectangle', status: TableStatus.reserved),
        DiningTable(id: 'tbl-t4', floorId: 'fl-main', tableCode: 'T4', capacity: 6, posX: 484, posY: 24, shape: 'rectangle', status: TableStatus.available),
        DiningTable(id: 'tbl-t5', floorId: 'fl-main', tableCode: 'T5', capacity: 2, posX: 24, posY: 164, shape: 'circle', status: TableStatus.dirty),
        DiningTable(id: 'tbl-t6', floorId: 'fl-main', tableCode: 'T6', capacity: 4, posX: 164, posY: 164, shape: 'rectangle', status: TableStatus.available),
        DiningTable(id: 'tbl-t7', floorId: 'fl-main', tableCode: 'T7', capacity: 4, posX: 324, posY: 164, shape: 'rectangle', status: TableStatus.available),
        DiningTable(id: 'tbl-t8', floorId: 'fl-main', tableCode: 'T8', capacity: 6, posX: 484, posY: 164, shape: 'booth', status: TableStatus.available),
      ],
    ),
    DiningFloor(
      id: 'fl-patio',
      name: 'Outdoor Patio',
      sortOrder: 2,
      tables: [
        DiningTable(id: 'tbl-p1', floorId: 'fl-patio', tableCode: 'P1', capacity: 2, posX: 24, posY: 24, shape: 'circle', status: TableStatus.available),
        DiningTable(id: 'tbl-p2', floorId: 'fl-patio', tableCode: 'P2', capacity: 4, posX: 164, posY: 24, shape: 'rectangle', status: TableStatus.available),
        DiningTable(id: 'tbl-p3', floorId: 'fl-patio', tableCode: 'P3', capacity: 4, posX: 324, posY: 24, shape: 'rectangle', status: TableStatus.available),
        DiningTable(id: 'tbl-p4', floorId: 'fl-patio', tableCode: 'P4', capacity: 6, posX: 484, posY: 24, shape: 'rectangle', status: TableStatus.available),
      ],
    ),
  ];

  static const List<Category> categories = [
    Category(id: 'cat-espresso', name: 'Espresso & Coffee', sortOrder: 1),
    Category(id: 'cat-cold', name: 'Cold Brew', sortOrder: 2),
    Category(id: 'cat-bakery', name: 'Pastries & Bakery', sortOrder: 3),
    Category(id: 'cat-kitchen', name: 'Hot Kitchen', sortOrder: 4),
    Category(id: 'cat-cold-prep', name: 'Salads & Desserts', sortOrder: 5),
    Category(id: 'cat-retail', name: 'Retail Beans', sortOrder: 6),
  ];

  static final List<ModifierGroup> modifierGroups = [
    ModifierGroup(
      id: 'mg-milk',
      name: 'Milk Choice',
      minSelections: 1,
      maxSelections: 1,
      modifiers: [
        Modifier(
          id: 'mod-whole',
          modifierGroupId: 'mg-milk',
          name: 'Whole Milk',
          priceDelta: Decimal.zero,
          isAvailable: true,
        ),
        Modifier(
          id: 'mod-oat',
          modifierGroupId: 'mg-milk',
          name: 'Barista Oat Milk',
          priceDelta: Decimal.parse('150.00'),
          isAvailable: true,
        ),
        Modifier(
          id: 'mod-almond',
          modifierGroupId: 'mg-milk',
          name: 'Almond Milk',
          priceDelta: Decimal.parse('180.00'),
          isAvailable: true,
        ),
      ],
    ),
    ModifierGroup(
      id: 'mg-shots',
      name: 'Espresso Shots',
      minSelections: 0,
      maxSelections: 2,
      modifiers: [
        Modifier(
          id: 'mod-extra-shot',
          modifierGroupId: 'mg-shots',
          name: 'Extra Shot',
          priceDelta: Decimal.parse('220.00'),
          isAvailable: true,
        ),
        Modifier(
          id: 'mod-decaf',
          modifierGroupId: 'mg-shots',
          name: 'Decaf Ristretto',
          priceDelta: Decimal.zero,
          isAvailable: true,
        ),
      ],
    ),
    ModifierGroup(
      id: 'mg-temp',
      name: 'Temperature',
      minSelections: 1,
      maxSelections: 1,
      modifiers: [
        Modifier(
          id: 'mod-hot',
          modifierGroupId: 'mg-temp',
          name: 'Hot',
          priceDelta: Decimal.zero,
          isAvailable: true,
        ),
        Modifier(
          id: 'mod-iced',
          modifierGroupId: 'mg-temp',
          name: 'Iced',
          priceDelta: Decimal.parse('40.00'),
          isAvailable: true,
        ),
      ],
    ),
  ];

  static final List<MenuItem> menuItems = [
    MenuItem(
      id: 'item-oat-latte',
      categoryId: 'cat-espresso',
      sku: 'COF-OAT-LG',
      name: 'Large Oat Latte',
      description: 'Double ristretto with silky steamed barista oat milk (16oz)',
      basePrice: Decimal.parse('950.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'barista',
      isAvailable: true,
      modifierGroupIds: const ['mg-shots', 'mg-temp'],
    ),
    MenuItem(
      id: 'item-flat-white',
      categoryId: 'cat-espresso',
      sku: 'COF-FW-REG',
      name: 'Flat White',
      description: 'Velvety microfoam over double house espresso',
      basePrice: Decimal.parse('650.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'barista',
      isAvailable: true,
      modifierGroupIds: const ['mg-milk', 'mg-shots'],
    ),
    MenuItem(
      id: 'item-cold-brew',
      categoryId: 'cat-cold',
      sku: 'CLD-BRW-16',
      name: '18h Cold Brew Tonic',
      description: 'Single-origin Ethiopian cold brew over artisanal tonic',
      basePrice: Decimal.parse('780.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'barista',
      isAvailable: true,
    ),
    MenuItem(
      id: 'item-croissant',
      categoryId: 'cat-bakery',
      sku: 'BAK-CRO-01',
      name: 'Butter Croissant',
      description: 'Flaky French-butter laminated pastry baked fresh',
      basePrice: Decimal.parse('450.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'pastry',
      isAvailable: true,
    ),
    MenuItem(
      id: 'item-cinnamon-roll',
      categoryId: 'cat-bakery',
      sku: 'BAK-CIN-02',
      name: 'Cardamom Cinnamon Roll',
      description: 'Warm brioche swirl with cream cheese glaze',
      basePrice: Decimal.parse('520.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'pastry',
      isAvailable: false,
    ),
    MenuItem(
      id: 'item-pasta',
      categoryId: 'cat-kitchen',
      sku: 'KIT-PAS-01',
      name: 'Truffle Mushroom Pasta',
      description: 'Hand-rolled tagliatelle with wild porcini and black truffle cream',
      basePrice: Decimal.parse('1850.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'hot_kitchen',
      isAvailable: true,
    ),
    MenuItem(
      id: 'item-pizza',
      categoryId: 'cat-kitchen',
      sku: 'KIT-PIZ-02',
      name: 'Wood-Fired Margherita',
      description: 'San Marzano tomato, fior di latte mozzarella, fresh basil',
      basePrice: Decimal.parse('1650.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'hot_kitchen',
      isAvailable: true,
    ),
    MenuItem(
      id: 'item-burrata',
      categoryId: 'cat-cold-prep',
      sku: 'CLD-BUR-01',
      name: 'Burrata Salad',
      description: 'Heirloom tomatoes, creamy Pugliese burrata, aged balsamic',
      basePrice: Decimal.parse('1400.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'cold_prep',
      isAvailable: true,
    ),
    MenuItem(
      id: 'item-tiramisu',
      categoryId: 'cat-cold-prep',
      sku: 'CLD-TIR-02',
      name: 'Classic Espresso Tiramisu',
      description: 'House espresso-soaked savoiardi with mascarpone sabayon',
      basePrice: Decimal.parse('850.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'cold_prep',
      isAvailable: true,
    ),
    MenuItem(
      id: 'item-beans-250',
      categoryId: 'cat-retail',
      sku: 'RET-ESP-250',
      name: 'House Espresso Beans 250g',
      description: 'Medium-dark roast, notes of dark chocolate & hazelnut',
      basePrice: Decimal.parse('2400.00'),
      taxRatePercent: Decimal.parse('16'),
      prepStation: 'expediter',
      isAvailable: true,
    ),
  ];
}
