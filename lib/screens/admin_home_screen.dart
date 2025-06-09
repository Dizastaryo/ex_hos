import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'profile_screen.dart';
import 'admin_management_screen.dart';
import 'admin_doctor_rooms_screen.dart';
import '../services/admin_service.dart';
import '../services/doctor_room_service.dart';

class AdminHomeScreen extends StatefulWidget {
  const AdminHomeScreen({Key? key}) : super(key: key);

  @override
  State<AdminHomeScreen> createState() => _AdminHomeScreenState();
}

class _AdminHomeScreenState extends State<AdminHomeScreen> {
  int _currentPage = 0;

  static const Color primaryColor = Color(0xFF30D5C8);

  @override
  Widget build(BuildContext context) {
    // Получаем сервисы через Provider
    final userService = Provider.of<UserService>(context, listen: false);
    final doctorRoomService =
        Provider.of<DoctorRoomService>(context, listen: false);

    final List<Widget> pages = [
      AdminDoctorRoomsPage(
        userService: userService,
        doctorRoomService: doctorRoomService,
      ),
      UserManagementScreen(),
      ProfileScreen(),
    ];

    return Scaffold(
      appBar: AppBar(
        title: const Text('Панель администратора'),
        centerTitle: true,
        backgroundColor: primaryColor,
        elevation: 4,
        automaticallyImplyLeading: false,
      ),
      body: AnimatedSwitcher(
        duration: const Duration(milliseconds: 300),
        child: pages[_currentPage],
        switchInCurve: Curves.easeIn,
        switchOutCurve: Curves.easeOut,
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _currentPage,
        selectedItemColor: primaryColor,
        unselectedItemColor: Colors.grey.shade600,
        showUnselectedLabels: true,
        type: BottomNavigationBarType.fixed,
        onTap: (index) => setState(() => _currentPage = index),
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.meeting_room_outlined),
            activeIcon: Icon(Icons.meeting_room),
            label: 'Кабинеты',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.supervised_user_circle_outlined),
            activeIcon: Icon(Icons.supervised_user_circle),
            label: 'Управление',
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
