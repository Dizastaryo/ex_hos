import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/appointment_service.dart';
import '../services/admin_service.dart';
import '../services/chat_service.dart';
import 'profile_screen.dart';
import 'doctor_appointments_screen.dart';

class ModeratorHomeScreen extends StatefulWidget {
  const ModeratorHomeScreen({Key? key}) : super(key: key);

  @override
  _ModeratorHomeScreenState createState() => _ModeratorHomeScreenState();
}

class _ModeratorHomeScreenState extends State<ModeratorHomeScreen> {
  int _currentPage = 0;

  @override
  Widget build(BuildContext context) {
    final appointmentService =
        Provider.of<AppointmentService>(context, listen: false);
    final userService = Provider.of<UserService>(context, listen: false);
    final chatService = Provider.of<ChatService>(context, listen: false);

    final List<Widget> _pages = [
      DoctorAppointmentsPage(
        appointmentService: appointmentService,
        userService: userService,
        chatService: chatService,
      ),
      ProfileScreen(),
    ];

    const Color primaryColor = Color(0xFF30D5C8);

    return Scaffold(
      appBar: AppBar(
        title: const Text(
          'Панель модератора',
          style: TextStyle(
            fontWeight: FontWeight.bold,
            fontSize: 20,
            letterSpacing: 1.1,
            fontFamily: 'Roboto',
          ),
        ),
        centerTitle: true,
        backgroundColor: primaryColor,
        elevation: 0,
        automaticallyImplyLeading: false,
        flexibleSpace: Container(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              colors: [primaryColor, primaryColor.withOpacity(0.8)],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
          ),
        ),
      ),
      body: AnimatedSwitcher(
        duration: const Duration(milliseconds: 500),
        transitionBuilder: (Widget child, Animation<double> animation) {
          return FadeTransition(opacity: animation, child: child);
        },
        child: _pages[_currentPage],
      ),
      bottomNavigationBar: Container(
        decoration: BoxDecoration(
          color: Colors.white,
          boxShadow: [
            BoxShadow(
              color: Colors.black.withOpacity(0.1),
              blurRadius: 10,
              offset: const Offset(0, -2),
            ),
          ],
        ),
        child: BottomNavigationBar(
          backgroundColor: Colors.white,
          currentIndex: _currentPage,
          selectedItemColor: primaryColor,
          unselectedItemColor: Colors.grey[600],
          showUnselectedLabels: true,
          type: BottomNavigationBarType.fixed,
          onTap: (index) => setState(() => _currentPage = index),
          items: const [
            BottomNavigationBarItem(
              icon: Icon(Icons.event_note_outlined),
              activeIcon: Icon(Icons.event_note),
              label: 'Записи',
            ),
            BottomNavigationBarItem(
              icon: Icon(Icons.person_outline),
              activeIcon: Icon(Icons.person),
              label: 'Профиль',
            ),
          ],
          selectedLabelStyle: const TextStyle(fontWeight: FontWeight.bold),
          unselectedLabelStyle: const TextStyle(fontWeight: FontWeight.normal),
        ),
      ),
    );
  }
}
