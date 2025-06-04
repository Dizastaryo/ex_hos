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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(
          "Свободные слоты на ${selectedDate.toLocal().toString().split(' ')[0]}",
        ),
      ),
      body: FutureBuilder<List<dynamic>>(
        future: service.getSlots(roomNumber, selectedDate),
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(child: Text("Ошибка: ${snapshot.error}"));
          }

          final slots = snapshot.data!;

          if (slots.isEmpty) {
            return const Center(child: Text("Нет доступных слотов"));
          }

          return GridView.builder(
            padding: const EdgeInsets.all(16),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 3,
              crossAxisSpacing: 12,
              mainAxisSpacing: 12,
              childAspectRatio: 2.5,
            ),
            itemCount: slots.length,
            itemBuilder: (context, index) {
              final slot = slots[index];
              final time = slot.split(' ')[1].substring(0, 5); // "HH:MM"

              return ElevatedButton(
                style: ElevatedButton.styleFrom(
                  backgroundColor: Colors.teal.shade100,
                  foregroundColor: Colors.black,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
                onPressed: () {
                  showDialog(
                    context: context,
                    builder: (context) => AlertDialog(
                      title: Text('Подтверждение'),
                      content: Text('Вы хотите записаться на $time?'),
                      actions: [
                        TextButton(
                          onPressed: () => Navigator.of(context).pop(),
                          child: const Text('Отмена'),
                        ),
                        TextButton(
                          onPressed: () async {
                            Navigator.of(context).pop(); // Закрываем диалог
                            try {
                              final result = await service.bookSlot(
                                  roomNumber, slot); // Полная дата и время
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(
                                  content: Text('Успешно записались на $time'),
                                  backgroundColor: Colors.green,
                                ),
                              );
                            } catch (e) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(
                                  content: Text('Ошибка при записи: $e'),
                                  backgroundColor: Colors.red,
                                ),
                              );
                            }
                          },
                          child: const Text('Записаться'),
                        ),
                      ],
                    ),
                  );
                },
                child: Text(
                  time,
                  style: const TextStyle(fontWeight: FontWeight.bold),
                ),
              );
            },
          );
        },
      ),
    );
  }
}
