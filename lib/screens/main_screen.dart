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
    final dio = Provider.of<Dio>(context, listen: false);
    final appointmentService = AppointmentService(dio);

    return Scaffold(
      backgroundColor: Colors.grey[100],
      appBar: AppBar(
        title: const Text(
          "Запись на приём",
          style: TextStyle(fontWeight: FontWeight.bold),
        ),
        centerTitle: true,
        automaticallyImplyLeading: false,
        backgroundColor: const Color(0xFF30D5C8),
        elevation: 4,
      ),
      body: Padding(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            _buildCardButton(
              context,
              icon: Icons.local_hospital,
              title: "Кабинеты врачей",
              onTap: () {
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) =>
                        DoctorRoomsPage(service: appointmentService),
                  ),
                );
              },
            ),
            const SizedBox(height: 20),
            _buildCardButton(
              context,
              icon: Icons.biotech,
              title: "Кабинеты для анализов",
              onTap: () {
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) => TestRoomsPage(service: appointmentService),
                  ),
                );
              },
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCardButton(
    BuildContext context, {
    required IconData icon,
    required String title,
    required VoidCallback onTap,
  }) {
    return GestureDetector(
      onTap: onTap,
      child: Card(
        elevation: 6,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
        color: Colors.white,
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: 20, horizontal: 16),
          width: double.infinity,
          child: Row(
            children: [
              Container(
                decoration: BoxDecoration(
                  color: const Color(0xFF30D5C8),
                  borderRadius: BorderRadius.circular(12),
                ),
                padding: const EdgeInsets.all(12),
                child: Icon(icon, color: Colors.white, size: 28),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: Text(
                  title,
                  style: const TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              const Icon(Icons.arrow_forward_ios, size: 20),
            ],
          ),
        ),
      ),
    );
  }
}
