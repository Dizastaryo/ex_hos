import 'package:flutter/material.dart';
import '../services/appointment_service.dart';

class SlotsPage extends StatelessWidget {
  final AppointmentService service;
  final int roomNumber;
  final DateTime selectedDate;

  const SlotsPage({
    super.key,
    required this.service,
    required this.roomNumber,
    required this.selectedDate,
  });

  Future<void> _bookSlot(BuildContext context, String slot, String time) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text("Подтверждение"),
        content: Text("Вы хотите записаться на $time?"),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text("Отмена"),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(context).pop(true),
            style: ElevatedButton.styleFrom(
              backgroundColor: const Color(0xFF30d5c8),
              foregroundColor: Colors.white,
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8)),
            ),
            child: const Text("Записаться"),
          ),
        ],
      ),
    );

    if (confirmed != true) return;

    try {
      await service.bookSlot(roomNumber, slot);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text("Успешно записались на $time"),
          backgroundColor: Colors.green,
        ),
      );
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text("Ошибка при записи: $e"),
          backgroundColor: Colors.red,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final dateStr =
        "${selectedDate.year}-${selectedDate.month.toString().padLeft(2, '0')}-${selectedDate.day.toString().padLeft(2, '0')}";

    return Scaffold(
      appBar: AppBar(
        title: Text("Слоты на $dateStr"),
        backgroundColor: const Color(0xFF30d5c8),
        foregroundColor: Colors.white,
        elevation: 2,
      ),
      body: FutureBuilder<List<dynamic>>(
        future: service.getSlots(roomNumber, selectedDate),
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(
              child: CircularProgressIndicator(color: Color(0xFF30d5c8)),
            );
          }

          if (snapshot.hasError) {
            return Center(child: Text("Ошибка: ${snapshot.error}"));
          }

          final slots = snapshot.data!;
          if (slots.isEmpty) {
            return const Center(child: Text("Нет доступных слотов"));
          }

          return Padding(
            padding: const EdgeInsets.all(16),
            child: GridView.builder(
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 3,
                crossAxisSpacing: 14,
                mainAxisSpacing: 14,
                childAspectRatio: 2.6,
              ),
              itemCount: slots.length,
              itemBuilder: (context, index) {
                final slot = slots[index];
                final time = slot.split(' ')[1].substring(0, 5); // "HH:MM"

                return ElevatedButton(
                  onPressed: () => _bookSlot(context, slot, time),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF30d5c8),
                    foregroundColor: Colors.white,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(14),
                    ),
                    elevation: 3,
                    shadowColor: Colors.teal.withOpacity(0.3),
                    textStyle: const TextStyle(
                        fontWeight: FontWeight.bold, fontSize: 16),
                  ),
                  child: Text(time),
                );
              },
            ),
          );
        },
      ),
    );
  }
}
