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
  _AdminHomeScreenState createState() => _AdminHomeScreenState();
}

class _AdminHomeScreenState extends State<AdminHomeScreen> {
  int _currentPage = 0;

  @override
  Widget build(BuildContext context) {
    // Получаем зависимости через Provider
    final userService = Provider.of<UserService>(context, listen: false);
    final doctorRoomService =
        Provider.of<DoctorRoomService>(context, listen: false);

    final List<Widget> _pages = [
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
