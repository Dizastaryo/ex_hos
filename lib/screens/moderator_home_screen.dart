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

    return Scaffold(
      appBar: AppBar(
        title: const Text('Панель модератора'),
        centerTitle: true,
        backgroundColor: Color(0xFF6A0DAD),
        automaticallyImplyLeading: false,
      ),
      body: AnimatedSwitcher(
        duration: const Duration(milliseconds: 300),
        child: _pages[_currentPage],
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _currentPage,
        selectedItemColor: Color(0xFF6A0DAD),
        unselectedItemColor: Colors.grey,
        showUnselectedLabels: true,
        type: BottomNavigationBarType.fixed,
        onTap: (index) {
          setState(() {
            _currentPage = index;
          });
        },
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.shop_outlined),
            activeIcon: Icon(Icons.shop),
            label: 'Записы',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.account_circle_outlined),
            activeIcon: Icon(Icons.account_circle),
            label: 'Профиль',
          ),
        ],
      ),
    );
  }
}
