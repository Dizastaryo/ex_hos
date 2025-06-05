import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:dio/dio.dart';

import '../services/appointment_service.dart';
import 'doctor_rooms_page.dart';
import 'test_rooms_page.dart';

class MainScreen extends StatelessWidget {
  const MainScreen({super.key});

  @override
  Widget build(BuildContext context) {
    // Получаем общий Dio из провайдера
    final dio = Provider.of<Dio>(context, listen: false);
    final appointmentService = AppointmentService(dio);

    return Scaffold(
      appBar: AppBar(title: const Text("Запись на приём")),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            ElevatedButton.icon(
              icon: const Icon(Icons.local_hospital),
              onPressed: () {
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) =>
                        DoctorRoomsPage(service: appointmentService),
                  ),
                );
              },
              label: const Text("Кабинеты врачей"),
              style: ElevatedButton.styleFrom(
                minimumSize: const Size.fromHeight(50),
              ),
            ),
            const SizedBox(height: 20),
            ElevatedButton.icon(
              icon: const Icon(Icons.biotech),
              onPressed: () {
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) => TestRoomsPage(service: appointmentService),
                  ),
                );
              },
              label: const Text("Кабинеты для анализов"),
              style: ElevatedButton.styleFrom(
                minimumSize: const Size.fromHeight(50),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
