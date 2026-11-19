import 'package:flutter/material.dart';
import '../controllers/rider_controller.dart';
import 'active_trip_screen.dart';

class RiderHomeScreen extends StatelessWidget {
  const RiderHomeScreen({super.key, required this.controller});

  final RiderController controller;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Rider Dispatch', style: TextStyle(fontWeight: FontWeight.w800)),
        actions: [
          Switch(
            value: controller.onDuty,
            onChanged: (_) => controller.toggleDuty(),
          ),
          Padding(
            padding: const EdgeInsets.only(right: 12),
            child: Center(
              child: Text(
                controller.onDuty ? 'ON DUTY' : 'OFF',
                style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w700),
              ),
            ),
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          // Connectivity & Offline Buffer Status Bar
          if (!controller.isOnline || controller.offlineBuffer.isNotEmpty)
            Container(
              margin: const EdgeInsets.only(bottom: 12),
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: const Color(0xFFF59E0B).withValues(alpha: 0.16),
                borderRadius: BorderRadius.circular(10),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    'Buffered Offline (${controller.offlineBuffer.length} pending)',
                    style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 12),
                  ),
                  TextButton(
                    onPressed: () {
                      controller.setOnline(true);
                      controller.flushBufferedEvents();
                    },
                    child: const Text('Sync Now'),
                  ),
                ],
              ),
            ),

          // Cash-in-Hand COD Wallet Card
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: const Color(0xFF0F172A),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'CASH-IN-HAND (UNRECONCILED COD)',
                  style: TextStyle(color: Colors.white70, fontSize: 11, fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: 6),
                Text(
                  'PKR ${controller.unreconciledCodCashInHand.toStringAsFixed(2)}',
                  style: const TextStyle(color: Colors.white, fontSize: 24, fontWeight: FontWeight.w800),
                ),
                const SizedBox(height: 4),
                const Text(
                  'Settle with branch cashier at shift end',
                  style: TextStyle(color: Colors.white60, fontSize: 11),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),

          // Assigned Trips
          for (final trip in controller.trips)
            Card(
              margin: const EdgeInsets.only(bottom: 10),
              child: ListTile(
                onTap: () => Navigator.of(context).push(MaterialPageRoute(
                  builder: (_) => ActiveTripScreen(controller: controller, tripId: trip.tripId),
                )),
                title: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(trip.orderNumber, style: const TextStyle(fontWeight: FontWeight.w800)),
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(
                        color: trip.isCod
                            ? const Color(0xFFF59E0B).withValues(alpha: 0.16)
                            : const Color(0xFF10B981).withValues(alpha: 0.16),
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        trip.isCod ? 'COLLECT COD PKR ${trip.codExpected.toStringAsFixed(0)}' : 'PREPAID',
                        style: TextStyle(
                          fontSize: 10,
                          fontWeight: FontWeight.w800,
                          color: trip.isCod ? const Color(0xFFB45309) : const Color(0xFF047857),
                        ),
                      ),
                    ),
                  ],
                ),
                subtitle: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const SizedBox(height: 4),
                    Text('${trip.customerName} · ${trip.address}', style: const TextStyle(fontSize: 12)),
                    Text(
                      'Status: ${trip.status.toUpperCase()}',
                      style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: Color(0xFFEA580C)),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}
