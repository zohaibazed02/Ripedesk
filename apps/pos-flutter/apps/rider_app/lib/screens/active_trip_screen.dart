import 'package:flutter/material.dart';
import '../controllers/rider_controller.dart';

class ActiveTripScreen extends StatefulWidget {
  const ActiveTripScreen({
    super.key,
    required this.controller,
    required this.tripId,
  });

  final RiderController controller;
  final String tripId;

  @override
  State<ActiveTripScreen> createState() => _ActiveTripScreenState();
}

class _ActiveTripScreenState extends State<ActiveTripScreen> {
  final _recipientCtrl = TextEditingController();

  @override
  void dispose() {
    _recipientCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.controller,
      builder: (_, __) {
        final trip = widget.controller.trips.firstWhere((t) => t.tripId == widget.tripId);
        return Scaffold(
          appBar: AppBar(title: Text('Trip ${trip.orderNumber}')),
          body: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(trip.customerName, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
                Text(trip.address, style: const TextStyle(fontSize: 13)),
                const SizedBox(height: 12),
                Text(
                  'Current Status: ${trip.status.toUpperCase()}',
                  style: const TextStyle(fontWeight: FontWeight.w700, color: Color(0xFFEA580C)),
                ),
                const SizedBox(height: 20),
                if (trip.status == 'assigned')
                  ElevatedButton(
                    onPressed: () => widget.controller.advanceTripStatus(trip.tripId, 'picked_up'),
                    child: const Text('Confirm Pickup from Branch'),
                  ),
                if (trip.status == 'picked_up')
                  ElevatedButton(
                    onPressed: () => widget.controller.advanceTripStatus(trip.tripId, 'out_for_delivery'),
                    child: const Text('Start Delivery (Out for Delivery)'),
                  ),
                if (trip.status == 'out_for_delivery') ...[
                  TextField(
                    controller: _recipientCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Recipient Name (Proof of Delivery)',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 12),
                  ElevatedButton(
                    onPressed: () async {
                      await widget.controller.advanceTripStatus(
                        trip.tripId,
                        'delivered',
                        podPhotoUrl: 'pod://${trip.tripId}.jpg',
                        recipientName: _recipientCtrl.text.trim().isEmpty ? trip.customerName : _recipientCtrl.text.trim(),
                        codCollected: trip.codExpected,
                      );
                      if (context.mounted) Navigator.of(context).pop();
                    },
                    child: Text(
                      trip.isCod
                          ? 'Complete Delivery & Collect PKR ${trip.codExpected.toStringAsFixed(0)}'
                          : 'Complete Delivery (POD Captured)',
                    ),
                  ),
                ],
              ],
            ),
          ),
        );
      },
    );
  }
}
