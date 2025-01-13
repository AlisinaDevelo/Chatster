import React from 'react';
import { FaComments } from 'react-icons/fa';

const Header = () => {
  return (
    <div className="bg-gradient-to-r from-primary to-secondary text-white py-4 px-6 rounded-b-lg shadow-lg mb-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <FaComments className="text-2xl" />
          <h1 className="text-2xl font-bold tracking-tight m-0">Chatster</h1>
        </div>
        <span className="bg-white bg-opacity-20 px-3 py-1 rounded-full text-sm">Live Chat</span>
      </div>
    </div>
  );
};

export default Header;

