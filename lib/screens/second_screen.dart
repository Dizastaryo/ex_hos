import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';

class SecondScreen extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    // Calling autoLogin method when the screen is built
    Future.delayed(Duration.zero, () {
      context.read<AuthProvider>().autoLogin(context);
    });

    return Scaffold(
      backgroundColor: Colors.white, // Red background
      appBar: AppBar(
        title: Text(""),
        backgroundColor: Colors.white, // Match the background color with AppBar
      ),
      body: Center(
        child:
            CircularProgressIndicator(), // Show loading indicator while auto login
      ),
    );
  }
}
